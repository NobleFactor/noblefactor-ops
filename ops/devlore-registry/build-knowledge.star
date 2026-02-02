# SPDX-License-Identifier: MIT
# Copyright (c) 2025-2026 Noble Factor. All rights reserved.
#
# build-knowledge.star - Build knowledge base from devlore-cli source
#
# This is a build step that:
# 1. Interrogates devlore-cli source code (static analysis)
# 2. Enforces contracts (fails build on violations)
# 3. Rebuilds knowledge artifacts in devlore-registry
#
# Targets:
#   onboarding - Starlark bindings for lore onboard
#   migration  - Writ migrate patterns for writ migrate
#   all        - Both targets

def run(ctx):
    """Main entry point for build-knowledge command."""
    domain = ctx.args.get("domain", "all")
    source_path = ctx.args.get("source_path", "")
    registry_path = ctx.args.get("registry_path", "")

    # Smart defaults: look for sibling directories
    if not source_path:
        source_path = _find_sibling("devlore-cli")
        if source_path:
            note("Using sibling source: " + source_path)
        else:
            fail("--source-path required (no ../devlore-cli found)")

    if not registry_path:
        registry_path = _find_sibling("devlore-registry")
        if registry_path:
            note("Using sibling registry: " + registry_path)
        else:
            fail("--registry-path required (no ../devlore-registry found)")

    # Validate paths exist
    if not fs.is_dir(source_path):
        fail("Source path not found: " + source_path)
    if not fs.is_dir(registry_path):
        fail("Registry path not found: " + registry_path)

    # Run requested domains
    if domain == "all" or domain == "onboarding":
        build_onboarding_knowledge(source_path, registry_path)

    if domain == "all" or domain == "migration":
        build_migration_knowledge(source_path, registry_path)


def _find_sibling(name):
    """Find a sibling directory by name."""
    # Try ../name relative to current directory
    sibling = fs.join("..", name)
    if fs.is_dir(sibling):
        return sibling
    return ""


# =============================================================================
# ONBOARDING KNOWLEDGE (Starlark bindings for lore onboard)
# =============================================================================

def build_onboarding_knowledge(source_path, registry_path):
    """Build onboarding knowledge from Starlark bindings."""
    note("Building onboarding knowledge...")

    starlark_path = fs.join(source_path, "internal", "starlark")
    if not fs.is_dir(starlark_path):
        fail("Starlark source not found: " + starlark_path)

    knowledge_path = fs.join(registry_path, "knowledge", "package-authoring", "bindings")
    reference_path = fs.join(knowledge_path, "reference.yaml")
    rules_path = fs.join(knowledge_path, "rules.yaml")

    # Step 1: Parse Go source files
    note("  Scanning " + starlark_path + "...")
    result = go.parse_starlark_bindings(starlark_path)

    bindings = list(result.bindings)
    namespaces = list(result.namespaces)

    note("  Found " + str(len(bindings)) + " bindings in " + str(len(namespaces)) + " namespaces")

    # Step 2: Check for contract violations (fails build)
    violations = check_binding_contract_violations(bindings)
    if violations:
        error("Contract violations detected:")
        for v in violations:
            error("  " + v["binding"] + " (" + v["file"] + ":" + str(v["line"]) + ")")
            error("    " + v["message"])
        fail("Fix contract violations before building knowledge")

    success("  No contract violations")

    # Step 3: Organize bindings by namespace
    binding_tree = organize_bindings(bindings, namespaces)

    # Step 4: Load current reference if exists
    current_bindings = {}
    if fs.exists(reference_path):
        current_content = fs.read(reference_path)
        current_ref = yaml.decode(current_content)
        current_bindings = extract_current_bindings(current_ref)

    # Step 5: Compare and report diff
    new_bindings, removed_bindings, unchanged = compare_bindings(binding_tree, current_bindings)

    if new_bindings:
        success("  New bindings: " + str(len(new_bindings)))
        for b in new_bindings:
            note("    + " + b)

    if removed_bindings:
        warn("  Removed bindings: " + str(len(removed_bindings)))
        for b in removed_bindings:
            note("    - " + b)

    if not new_bindings and not removed_bindings:
        success("  No changes detected")

    # Step 6: Generate new reference.yaml
    new_reference = generate_reference(binding_tree, bindings)
    reference_content = yaml.encode(new_reference)

    fs.write(reference_path, reference_content)
    success("  Wrote " + reference_path)

    # Step 7: Update rules.yaml with new bindings
    if new_bindings and fs.exists(rules_path):
        rules_content = fs.read(rules_path)
        rules = yaml.decode(rules_content)
        updated_rules = update_rules_with_new_bindings(rules, new_bindings)
        fs.write(rules_path, yaml.encode(updated_rules))
        success("  Updated " + rules_path)

    # Step 8: Validate rules against reference
    if fs.exists(rules_path):
        validate_rules(rules_path, binding_tree)


# =============================================================================
# MIGRATION KNOWLEDGE (Writ migrate patterns for writ migrate)
# =============================================================================

def build_migration_knowledge(source_path, registry_path):
    """Build migration knowledge from writ migrate source.

    This validates that the Go source constants match the registry knowledge:
    - SourceSystem constants should have corresponding signature files
    - EncryptionSystem constants should be documented
    - Platform names should match writ-structure.yaml
    """
    note("Building migration knowledge...")

    migrate_path = fs.join(source_path, "internal", "writ", "migrate")
    if not fs.is_dir(migrate_path):
        fail("Migrate source not found: " + migrate_path)

    knowledge_path = fs.join(registry_path, "knowledge", "migration")
    if not fs.is_dir(knowledge_path):
        fail("Migration knowledge path not found: " + knowledge_path)

    # Step 1: Parse Go source files
    note("  Scanning " + migrate_path + "...")
    result = go.parse_migrate_knowledge(migrate_path)

    source_systems = list(result.source_systems)
    encryption_systems = list(result.encryption_systems)
    repo_layers = list(result.repo_layers)
    platforms = list(result.platforms)

    note("  Found " + str(len(source_systems)) + " source systems")
    note("  Found " + str(len(encryption_systems)) + " encryption systems")
    note("  Found " + str(len(platforms)) + " platforms")

    # Step 2: Load registry signature files
    signatures_path = fs.join(knowledge_path, "signatures")
    registry_systems = []
    if fs.is_dir(signatures_path):
        for entry in fs.list_dir(signatures_path):
            if entry.name.endswith(".yaml"):
                system_name = entry.name.replace(".yaml", "")
                registry_systems.append(system_name)

    # Step 3: Load writ-structure.yaml for platform validation
    writ_structure_path = fs.join(knowledge_path, "concepts", "writ-structure.yaml")
    registry_platforms = []
    registry_platform_aliases = []
    if fs.exists(writ_structure_path):
        content = fs.read(writ_structure_path)
        structure = yaml.decode(content)
        segments = structure.get("naming", {}).get("segments", {})
        platform_list = segments.get("platforms", [])
        for p in platform_list:
            if "name" in p:
                registry_platforms.append(p["name"])
            # Also collect aliases
            aliases = p.get("aliases", [])
            for alias in aliases:
                registry_platform_aliases.append(alias)

    # Step 4: Check for contract violations (source vs registry consistency)
    violations = check_migration_contract_violations(
        source_systems,
        encryption_systems,
        platforms,
        registry_systems,
        registry_platforms,
        registry_platform_aliases,
    )

    if violations:
        error("Contract violations detected:")
        for v in violations:
            error("  " + v["type"] + ": " + v["message"])
        fail("Fix contract violations before building knowledge")

    success("  No contract violations")

    # Step 5: Generate/update systems reference file
    systems_ref = generate_systems_reference(source_systems, encryption_systems, repo_layers, platforms)
    systems_ref_path = fs.join(knowledge_path, "systems-reference.yaml")

    # Compare with existing
    changes_detected = False
    if fs.exists(systems_ref_path):
        current_content = fs.read(systems_ref_path)
        new_content = yaml.encode(systems_ref)
        if current_content != new_content:
            changes_detected = True
            note("  Changes detected in systems-reference.yaml")
    else:
        changes_detected = True
        note("  Creating new systems-reference.yaml")

    if changes_detected:
        fs.write(systems_ref_path, yaml.encode(systems_ref))
        success("  Wrote " + systems_ref_path)
    else:
        success("  No changes to systems-reference.yaml")

    # Step 6: Validate all signature files exist for source systems
    validate_signature_coverage(source_systems, signatures_path)


def check_migration_contract_violations(source_systems, encryption_systems, platforms, registry_systems, registry_platforms, registry_platform_aliases):
    """Check for contract violations between source code and registry.

    Contract:
      - Every SourceSystem constant (except 'unknown') should have a signature file
      - Registry platforms should match source platforms (including aliases)
    """
    violations = []

    # Get source system values (excluding unknown and native which don't need signatures)
    source_system_values = []
    for s in source_systems:
        val = str(s.value)
        if val not in ["unknown", "native"]:
            source_system_values.append(val)

    # Check: source systems should have registry signatures
    for system in source_system_values:
        if system not in registry_systems:
            violations.append({
                "type": "missing_signature",
                "message": "Source system '" + system + "' has no signature file in registry",
            })

    # Check: registry signatures should have source system constants
    for system in registry_systems:
        # Skip encryption systems (git-crypt is an encryption system, not a source system)
        if system in ["git-crypt", "sops", "age", "gpg", "blackbox", "transcrypt", "ansible-vault"]:
            continue
        found = False
        for s in source_systems:
            if str(s.value) == system:
                found = True
                break
        if not found:
            violations.append({
                "type": "orphan_signature",
                "message": "Registry signature '" + system + "' has no SourceSystem constant",
            })

    # Check: platforms in prompt should match registry platforms (or be an alias)
    # Aliases are case-insensitive (ubuntu is alias for Debian)
    if registry_platforms:
        all_valid_platforms = set(registry_platforms)
        for alias in registry_platform_aliases:
            all_valid_platforms.add(alias)
            all_valid_platforms.add(alias.lower())
            all_valid_platforms.add(alias.title())

        for platform in platforms:
            if platform not in all_valid_platforms and platform.lower() not in all_valid_platforms:
                violations.append({
                    "type": "undocumented_platform",
                    "message": "Platform '" + platform + "' in LLM prompt not in writ-structure.yaml",
                })

    return violations


def generate_systems_reference(source_systems, encryption_systems, repo_layers, platforms):
    """Generate systems-reference.yaml from Go source constants."""
    ref = {
        "version": "1.0",
        "source": "devlore-cli/internal/writ/migrate",
        "generated": True,
        "description": "Auto-generated reference of migrate constants from Go source",
    }

    # Source systems
    systems = []
    for s in source_systems:
        systems.append({
            "name": str(s.name),
            "value": str(s.value),
            "file": str(s.file),
            "line": int(s.line),
        })
    ref["source_systems"] = systems

    # Encryption systems
    encryptions = []
    for e in encryption_systems:
        encryptions.append({
            "name": str(e.name),
            "value": str(e.value),
            "file": str(e.file),
            "line": int(e.line),
        })
    ref["encryption_systems"] = encryptions

    # Repo layers
    layers = []
    for r in repo_layers:
        layers.append({
            "name": str(r.name),
            "value": str(r.value),
            "file": str(r.file),
            "line": int(r.line),
        })
    ref["repo_layers"] = layers

    # Platforms from LLM prompt
    ref["platforms"] = [str(p) for p in platforms]

    return ref


def validate_signature_coverage(source_systems, signatures_path):
    """Validate that all source systems have proper signature files."""
    for s in source_systems:
        val = str(s.value)
        if val in ["unknown", "native"]:
            continue

        sig_file = fs.join(signatures_path, val + ".yaml")
        if not fs.exists(sig_file):
            warn("  Missing signature file: " + sig_file)
        else:
            # Validate signature file has required fields
            content = fs.read(sig_file)
            sig = yaml.decode(content)
            if not sig.get("name"):
                warn("  Signature missing 'name': " + sig_file)
            if not sig.get("markers"):
                warn("  Signature missing 'markers': " + sig_file)


# =============================================================================
# CONTRACT VIOLATION DETECTION
# =============================================================================

def check_binding_contract_violations(bindings):
    """Check all bindings for contract violations.

    Contract:
      - package.* must be read-only (should NOT mutate)
      - system.* must be read-only (should NOT mutate)
      - plan.* must be execution graph builder (SHOULD mutate)
    """
    violations = []

    for b in bindings:
        full_name = b.full_name
        namespace = b.namespace
        mutates = b.mutates
        file = b.file
        line = int(b.line)

        top_level = namespace.split(".")[0] if namespace else full_name.split(".")[0]

        if top_level == "package":
            if mutates:
                violations.append({
                    "binding": full_name,
                    "file": file,
                    "line": line,
                    "message": "package.* bindings must be read-only but this one mutates",
                })

        elif top_level == "system":
            if mutates:
                violations.append({
                    "binding": full_name,
                    "file": file,
                    "line": line,
                    "message": "system.* bindings must be read-only but this one mutates",
                })

        elif top_level == "plan":
            if not mutates:
                violations.append({
                    "binding": full_name,
                    "file": file,
                    "line": line,
                    "message": "plan.* bindings must build execution graph but this one doesn't",
                })

    return violations


# =============================================================================
# BINDING ORGANIZATION AND COMPARISON
# =============================================================================

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

    methods = ns.get("methods", [])
    for m in methods:
        if "name" in m:
            full_name = ns_name + "." + m["name"]
            bindings[full_name] = m

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


# =============================================================================
# REFERENCE GENERATION
# =============================================================================

def generate_reference(binding_tree, bindings):
    """Generate new reference.yaml content."""
    ref = {
        "version": "1.0",
        "source": "devlore-cli/internal/starlark",
        "generated": True,
        "binding_count": len(bindings),
    }

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
            ref[top_level]["methods"] = data["methods"]
        else:
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
    """Add new bindings to rules.yaml as implemented."""
    binding_coverage = rules.get("binding_coverage", {})

    for binding_name in new_bindings:
        parts = binding_name.split(".")
        if len(parts) >= 2:
            category = parts[0]
            method = parts[-1]

            if category not in binding_coverage:
                binding_coverage[category] = {}

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

    all_bindings = set()
    for ns, data in binding_tree.items():
        for m in data["methods"]:
            all_bindings.add(m["full_name"])

    antipatterns = rules.get("shell_antipatterns", [])
    warnings = []

    for ap in antipatterns:
        correct = ap.get("correct_binding", "")
        if correct and not _binding_exists(correct, all_bindings):
            proposed = ap.get("proposed_binding", {})
            if not proposed:
                warnings.append("Rule references missing binding: " + correct)

    coverage = rules.get("binding_coverage", {})
    for category, methods in coverage.items():
        for method_name, method_data in methods.items():
            binding = ""
            status = ""
            if type(method_data) == "dict":
                binding = method_data.get("binding", "")
                status = method_data.get("status", "")
            if binding and status == "implemented" and not _binding_exists(binding, all_bindings):
                warnings.append("Coverage claims implemented but not found: " + binding)

    if warnings:
        for w in warnings:
            warn("    " + w)
    else:
        success("  All rule references are valid")


def _binding_exists(binding_name, all_bindings):
    """Check if a binding exists, handling wildcards."""
    if binding_name in all_bindings:
        return True

    if "*" in binding_name:
        prefix = binding_name.replace("*", "")
        for b in all_bindings:
            if b.startswith(prefix):
                return True

    return False


command(
    name = "devlore-registry.build.knowledge",
    help = "Build knowledge base from devlore-cli source",
    flags = [
        {"name": "domain", "help": "Domain: all, onboarding, migration", "default": "all"},
        {"name": "source_path", "help": "Path to devlore-cli (default: ../devlore-cli)", "default": ""},
        {"name": "registry_path", "help": "Path to devlore-registry (default: ../devlore-registry)", "default": ""},
    ],
    run = run,
)
