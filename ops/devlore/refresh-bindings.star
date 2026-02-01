# SPDX-License-Identifier: MIT
# Copyright (c) 2025-2026 Noble Factor. All rights reserved.
#
# refresh-bindings.star - Static analysis tool for devlore-cli Starlark bindings
#
# Primary function: Detect violations of the binding contract:
#   - package.* must be read-only (lore package context)
#   - system.* must be read-only (system state queries)
#   - plan.* must mutate (execution graph builder)
#
# Side effect: Updates the knowledge base (reference.yaml, rules.yaml)
#
# This script:
# 1. Parses devlore-cli/internal/starlark/*.go to extract binding definitions
# 2. Analyzes each binding for contract violations
# 3. Reports violations as errors
# 4. Updates reference.yaml and rules.yaml (if no violations)

def run(ctx):
    """Main entry point for refresh-bindings command."""
    cli_path = ctx.args.get("cli_path", "")
    registry_path = ctx.args.get("registry_path", "")
    dry_run = ctx.args.get("dry_run", "") == "true"

    if not cli_path:
        fail("--cli-path is required")
    if not registry_path:
        fail("--registry-path is required")

    starlark_path = fs.join(cli_path, "internal", "starlark")
    if not fs.is_dir(starlark_path):
        fail("Not found: " + starlark_path)

    knowledge_path = fs.join(registry_path, "knowledge", "package-authoring", "bindings")
    reference_path = fs.join(knowledge_path, "reference.yaml")
    rules_path = fs.join(knowledge_path, "rules.yaml")

    # Step 1: Parse Go source files
    note("Scanning " + starlark_path + "...")
    result = go.parse_starlark_bindings(starlark_path)

    bindings = list(result.bindings)
    namespaces = list(result.namespaces)

    note("  Found " + str(len(bindings)) + " bindings in " + str(len(namespaces)) + " namespaces")

    # Step 2: Check for contract violations (PRIMARY FUNCTION)
    violations = check_contract_violations(bindings)
    if violations:
        error("Contract violations detected:")
        for v in violations:
            error("  " + v["binding"] + " (" + v["file"] + ":" + str(v["line"]) + ")")
            error("    " + v["message"])
        fail("Fix contract violations before updating knowledge base")

    success("No contract violations")

    # Step 3: Organize bindings by namespace
    binding_tree = organize_bindings(bindings, namespaces)

    # Step 4: Load current reference if exists
    current_bindings = {}
    if fs.exists(reference_path):
        note("Loading current reference.yaml...")
        current_content = fs.read(reference_path)
        current_ref = yaml.decode(current_content)
        current_bindings = extract_current_bindings(current_ref)
        note("  Current reference has " + str(len(current_bindings)) + " bindings")

    # Step 5: Compare and generate diff
    new_bindings, removed_bindings, unchanged = compare_bindings(binding_tree, current_bindings)

    if new_bindings:
        success("New bindings found: " + str(len(new_bindings)))
        for b in new_bindings:
            note("  + " + b)

    if removed_bindings:
        warn("Removed bindings: " + str(len(removed_bindings)))
        for b in removed_bindings:
            note("  - " + b)

    if not new_bindings and not removed_bindings:
        success("No changes detected")

    # Step 6: Generate new reference.yaml (SIDE EFFECT)
    new_reference = generate_reference(binding_tree, bindings)
    reference_content = yaml.encode(new_reference)

    if dry_run:
        note("Dry run - would write to: " + reference_path)
        print("---")
        print(reference_content)
        print("---")
    else:
        fs.write(reference_path, reference_content)
        success("Wrote " + reference_path)

    # Step 7: Update rules.yaml with new bindings (SIDE EFFECT)
    if new_bindings and fs.exists(rules_path):
        note("Updating rules.yaml with new bindings...")
        rules_content = fs.read(rules_path)
        rules = yaml.decode(rules_content)

        updated_rules = update_rules_with_new_bindings(rules, new_bindings)

        if dry_run:
            note("Dry run - would update rules.yaml")
        else:
            fs.write(rules_path, yaml.encode(updated_rules))
            success("Updated " + rules_path)

    # Step 8: Validate rules against reference
    if fs.exists(rules_path):
        note("Validating rules.yaml...")
        validate_rules(rules_path, binding_tree)


# =============================================================================
# CONTRACT VIOLATION DETECTION
# =============================================================================

def check_contract_violations(bindings):
    """Check all bindings for contract violations.

    Contract:
      - package.* must be read-only (should NOT mutate)
      - system.* must be read-only (should NOT mutate)
      - plan.* must be execution graph builder (SHOULD mutate)

    Returns list of violations, each with:
      - binding: full binding name
      - file: source file
      - line: line number
      - message: violation description
    """
    violations = []

    for b in bindings:
        full_name = b.full_name
        namespace = b.namespace
        mutates = b.mutates
        file = b.file
        line = int(b.line)

        # Determine expected contract based on top-level namespace
        top_level = namespace.split(".")[0] if namespace else full_name.split(".")[0]

        if top_level == "package":
            # package.* must be read-only
            if mutates:
                violations.append({
                    "binding": full_name,
                    "file": file,
                    "line": line,
                    "message": "package.* bindings must be read-only but this one mutates (creates execution.Node)",
                })

        elif top_level == "system":
            # system.* must be read-only
            if mutates:
                violations.append({
                    "binding": full_name,
                    "file": file,
                    "line": line,
                    "message": "system.* bindings must be read-only but this one mutates (creates execution.Node)",
                })

        elif top_level == "plan":
            # plan.* SHOULD mutate (build execution graph)
            if not mutates:
                violations.append({
                    "binding": full_name,
                    "file": file,
                    "line": line,
                    "message": "plan.* bindings must build execution graph but this one doesn't create execution.Node",
                })

        # Other namespaces (fs, shell, git, docker, etc.) are allowed to have either behavior
        # These are utility namespaces used in ops scripts, not lore packages

    return violations


def organize_bindings(bindings, namespaces):
    """Organize bindings into a tree structure by namespace."""
    tree = {}

    for b in bindings:
        full_name = b.full_name
        namespace = b.namespace
        name = b.name

        if namespace not in tree:
            tree[namespace] = {
                "methods": [],
                "file": b.file,
            }

        tree[namespace]["methods"].append({
            "name": name,
            "full_name": full_name,
            "file": b.file,
            "line": int(b.line),
        })

    return tree


def extract_current_bindings(ref):
    """Extract binding names from current reference.yaml."""
    bindings = {}

    # Handle different possible structures
    if "package" in ref:
        _extract_namespace_bindings(ref, "package", bindings)
    if "system" in ref:
        _extract_namespace_bindings(ref, "system", bindings)
    if "plan" in ref:
        _extract_namespace_bindings(ref, "plan", bindings)

    return bindings


def _extract_namespace_bindings(ref, ns_name, bindings):
    """Extract bindings from a namespace in the reference."""
    ns = ref.get(ns_name, {})

    # Check for methods list
    methods = ns.get("methods", [])
    for m in methods:
        if "name" in m:
            full_name = ns_name + "." + m["name"]
            bindings[full_name] = m

    # Check for nested namespaces
    namespaces = ns.get("namespaces", {})
    for sub_name, sub_ns in namespaces.items():
        sub_methods = sub_ns.get("methods", [])
        for m in sub_methods:
            if "name" in m:
                full_name = ns_name + "." + sub_name + "." + m["name"]
                bindings[full_name] = m


def compare_bindings(new_tree, current_bindings):
    """Compare new bindings against current reference."""
    new_set = set()
    for ns, data in new_tree.items():
        for m in data["methods"]:
            new_set.add(m["full_name"])

    current_set = set(current_bindings.keys())

    new_bindings = sorted(list(new_set - current_set))
    removed_bindings = sorted(list(current_set - new_set))
    unchanged = sorted(list(new_set & current_set))

    return new_bindings, removed_bindings, unchanged


def generate_reference(binding_tree, bindings):
    """Generate new reference.yaml content."""
    # Build structured reference
    ref = {
        "version": "1.0",
        "source": "devlore-cli/internal/starlark",
        "generated": True,
        "binding_count": len(bindings),
    }

    # Organize by top-level namespace (package, system, plan)
    for ns, data in sorted(binding_tree.items()):
        parts = ns.split(".")
        top_level = parts[0]

        if top_level not in ref:
            ref[top_level] = {
                "description": _get_namespace_description(top_level),
                "namespaces": {},
                "methods": [],
            }

        if len(parts) == 1:
            # Top-level methods
            ref[top_level]["methods"] = data["methods"]
        else:
            # Nested namespace
            sub_ns = ".".join(parts[1:])
            ref[top_level]["namespaces"][sub_ns] = {
                "description": _get_namespace_description(ns),
                "methods": data["methods"],
            }

    return ref


def _get_namespace_description(ns):
    """Get description for a namespace."""
    descriptions = {
        "package": "Lore package context - read-only access to package metadata",
        "system": "Read-only system state queries",
        "system.platform": "Platform information",
        "system.package": "Package manager queries",
        "system.service": "Service/daemon queries",
        "plan": "Execution graph builder - all mutations go through plan",
        "plan.package": "Package management operations",
        "plan.file": "File operations",
        "fs": "File system operations",
        "shell": "Shell command execution",
        "http": "HTTP operations",
        "archive": "Archive operations",
        "env": "Environment variable operations",
        "service": "Service management",
        "git": "Git operations",
        "docker": "Docker operations",
        "log": "Logging functions",
    }
    return descriptions.get(ns, "")


def update_rules_with_new_bindings(rules, new_bindings):
    """Add new bindings to rules.yaml as proposed."""
    binding_coverage = rules.get("binding_coverage", {})

    for binding_name in new_bindings:
        parts = binding_name.split(".")
        if len(parts) >= 2:
            category = parts[0]  # e.g., "plan", "system"
            method = parts[-1]   # e.g., "install", "clone"

            if category not in binding_coverage:
                binding_coverage[category] = {}

            # Add as proposed if not already present
            if method not in binding_coverage[category]:
                binding_coverage[category][method] = {
                    "binding": binding_name,
                    "status": "implemented",
                    "replaces": [],
                }

    rules["binding_coverage"] = binding_coverage
    return rules


def validate_rules(rules_path, binding_tree):
    """Validate that rules.yaml only references existing bindings."""
    rules_content = fs.read(rules_path)
    rules = yaml.decode(rules_content)

    # Collect all binding names
    all_bindings = set()
    for ns, data in binding_tree.items():
        for m in data["methods"]:
            all_bindings.add(m["full_name"])

    # Check shell_antipatterns
    antipatterns = rules.get("shell_antipatterns", [])
    warnings = []

    for ap in antipatterns:
        correct = ap.get("correct_binding", "")
        if correct and not _binding_exists(correct, all_bindings):
            # Check if it's a proposed binding
            proposed = ap.get("proposed_binding", {})
            if not proposed:
                warnings.append("Rule references missing binding: " + correct)

    # Check binding_coverage
    coverage = rules.get("binding_coverage", {})
    for category, methods in coverage.items():
        for method_name, method_data in methods.items():
            # Check if method_data is a dict by checking type
            binding = ""
            status = ""
            if type(method_data) == "dict":
                binding = method_data.get("binding", "")
                status = method_data.get("status", "")
            if binding and status == "implemented" and not _binding_exists(binding, all_bindings):
                warnings.append("Coverage claims implemented but not found: " + binding)

    if warnings:
        for w in warnings:
            warn("  " + w)
    else:
        success("  All rule references are valid")


def _binding_exists(binding_name, all_bindings):
    """Check if a binding exists, handling wildcards."""
    if binding_name in all_bindings:
        return True

    # Handle patterns like plan.file.*
    if "*" in binding_name:
        prefix = binding_name.replace("*", "")
        for b in all_bindings:
            if b.startswith(prefix):
                return True

    return False


command(
    name = "devlore.refresh-bindings",
    help = "Refresh binding knowledge from devlore-cli source code",
    flags = [
        {"name": "cli_path", "help": "Path to devlore-cli repository", "required": True},
        {"name": "registry_path", "help": "Path to devlore-registry repository", "required": True},
        {"name": "dry_run", "help": "Preview changes without writing", "default": ""},
    ],
    run = run,
)
