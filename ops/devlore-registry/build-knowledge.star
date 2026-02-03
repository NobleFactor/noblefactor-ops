# SPDX-License-Identifier: MIT
# Copyright (c) 2025-2026 Noble Factor. All rights reserved.
#
# build-knowledge.star - Build knowledge artifacts from devlore-cli source
#
# This is a build step that:
# 1. Interrogates devlore-cli source code (static analysis via Go AST)
# 2. Enforces contracts (fails build on violations)
# 3. Rebuilds knowledge artifacts in devlore-registry
#
# Domains:
#   - onboarding: Starlark API reference for lore package authors
#   - migration: writ migrate patterns (source systems, encryption, execution ops)
#   - all: Both domains (default)

def run(ctx):
    """Main entry point for build-knowledge command."""
    source_path = ctx.args.get("source_path", "")
    registry_path = ctx.args.get("registry_path", "")
    domain = ctx.args.get("domain", "all")

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

    # Build knowledge for selected domain(s)
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
# ONBOARDING KNOWLEDGE (Starlark API reference for lore package authors)
# =============================================================================

def build_onboarding_knowledge(source_path, registry_path):
    """Build Starlark API reference from devlore-cli source.

    Uses go.parse_devlore_api() to extract the API from Go source code.
    Writes the hierarchical API reference to the registry.
    """
    note("Building onboarding knowledge (Starlark API)...")

    starlark_path = fs.join(source_path, "internal", "starlark")
    if not fs.is_dir(starlark_path):
        fail("Starlark package not found: " + starlark_path)

    # Parse the API using Go AST - returns hierarchical structure
    note("  Scanning " + starlark_path + "...")
    api = go.parse_devlore_api(starlark_path)

    # Count bindings
    binding_count = _count_bindings(api)
    violation_count = len(list(api.violations))

    note("  Found " + str(binding_count) + " bindings")

    # Check for contract violations
    if violation_count > 0:
        error("Contract violations detected:")
        for v in api.violations:
            error("  " + v.name + " (" + v.file + ":" + str(v.line) + "): " + v.error)
        fail("Fix contract violations before building knowledge")

    success("  No contract violations")

    # Convert to dict for YAML serialization
    api_dict = _api_to_dict(api)

    # Write to registry
    reference_path = fs.join(registry_path, "knowledge", "package-authoring", "bindings", "reference.yaml")

    # Compare with existing
    changes_detected = False
    new_content = yaml.encode(api_dict)
    if fs.exists(reference_path):
        current_content = fs.read(reference_path)
        if current_content != new_content:
            changes_detected = True
            note("  Changes detected in reference.yaml")
    else:
        changes_detected = True
        note("  Creating new reference.yaml")

    if changes_detected:
        fs.write(reference_path, new_content)
        success("  Wrote " + reference_path)
    else:
        success("  No changes to reference.yaml")


def _count_bindings(api):
    """Count total bindings in the hierarchical API."""
    count = 0
    for ns in dir(api.plan):
        if not ns.startswith("_"):
            count += len(list(getattr(api.plan, ns)))
    for ns in dir(api.system):
        if not ns.startswith("_"):
            count += len(list(getattr(api.system, ns)))
    return count


def _api_to_dict(api):
    """Convert API struct to dict for YAML serialization."""
    result = {
        "valid": bool(api.valid),
        "plan": {},
        "system": {},
        "violations": [],
    }

    # Convert plan namespaces
    for ns in dir(api.plan):
        if ns.startswith("_"):
            continue
        methods = list(getattr(api.plan, ns))
        result["plan"][ns] = [_method_to_dict(m) for m in methods]

    # Convert system namespaces
    for ns in dir(api.system):
        if ns.startswith("_"):
            continue
        methods = list(getattr(api.system, ns))
        result["system"][ns] = [_method_to_dict(m) for m in methods]

    # Convert violations
    for v in api.violations:
        result["violations"].append({
            "name": v.name,
            "file": v.file,
            "line": int(v.line),
            "error": v.error,
        })

    return result


def _method_to_dict(m):
    """Convert a method struct to dict."""
    # Convert slot_docs struct to dict
    slot_docs = {}
    for slot in m.slots:
        doc = getattr(m.slot_docs, slot, "")
        if doc:
            slot_docs[slot] = doc

    return {
        "name": m.name,
        "full_name": m.full_name,
        "doc": m.doc,
        "usage": m.usage,
        "slots": list(m.slots),
        "slot_docs": slot_docs,
        "operations": list(m.operations),
        "output": m.output,
        "returns": m.returns,
        "file": m.file,
        "line": int(m.line),
    }


# =============================================================================
# MIGRATION KNOWLEDGE (Writ migrate patterns for writ migrate)
# =============================================================================

def build_migration_knowledge(source_path, registry_path):
    """Build migration knowledge from writ migrate source.

    This validates that the Go source constants match the registry knowledge:
    - SourceSystem constants should have corresponding signature files
    - EncryptionSystem constants should be documented
    - Platform names should match writ-structure.yaml
    - Execution operations in schemas match ops.go
    """
    note("Building migration knowledge...")

    migrate_path = fs.join(source_path, "internal", "writ", "migrate")
    if not fs.is_dir(migrate_path):
        fail("Migrate source not found: " + migrate_path)

    execution_path = fs.join(source_path, "internal", "execution")
    if not fs.is_dir(execution_path):
        fail("Execution source not found: " + execution_path)

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

    # Step 1b: Parse execution operations from ops.go
    ops_path = fs.join(execution_path, "ops.go")
    note("  Scanning " + ops_path + "...")
    ops_result = go.parse_execution_ops(ops_path)
    execution_ops = list(ops_result.operations)
    note("  Found " + str(len(execution_ops)) + " execution operations")

    # Step 2: Load registry signature files
    # Only include files that look like actual system signatures (have a 'name' field)
    signatures_path = fs.join(knowledge_path, "signatures")
    registry_systems = []
    if fs.is_dir(signatures_path):
        for entry in fs.list_dir(signatures_path):
            if entry.name.endswith(".yaml"):
                sig_path = fs.join(signatures_path, entry.name)
                content = fs.read(sig_path)
                sig = yaml.decode(content)
                # Only consider files with a 'name' field as system signatures
                if sig.get("name"):
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

    # Step 7: Update schema files with execution operations from source
    update_execution_ops_schema(execution_ops, knowledge_path)


def update_execution_ops_schema(execution_ops, knowledge_path):
    """Update schema files with execution operations extracted from Go source.

    This ensures the schema enum always matches the actual engine implementation.
    """
    schemas_path = fs.join(knowledge_path, "schemas")
    if not fs.is_dir(schemas_path):
        warn("  Schemas path not found: " + schemas_path)
        return

    # Sort operations alphabetically for consistent output
    # execution_ops is a list of structs with .name field
    ops_list = sorted([op.name for op in execution_ops])

    # Update engine-graph.json
    engine_schema_path = fs.join(schemas_path, "engine-graph.json")
    if fs.exists(engine_schema_path):
        content = fs.read(engine_schema_path)
        schema = json.decode(content)

        # Navigate to operations enum: $defs.node.properties.operations.items.enum
        node_def = schema.get("$defs", {}).get("node", {})
        ops_prop = node_def.get("properties", {}).get("operations", {})
        items = ops_prop.get("items", {})

        current_enum = items.get("enum", [])
        if sorted(current_enum) != ops_list:
            note("  Updating engine-graph.json operations enum")
            note("    From: " + ", ".join(sorted(current_enum)))
            note("    To:   " + ", ".join(ops_list))
            items["enum"] = ops_list
            fs.write(engine_schema_path, json.encode_indent(schema, "  "))
            success("  Wrote " + engine_schema_path)
        else:
            success("  engine-graph.json operations enum is up to date")

    # Update migration-plan.json (if it has an operations enum)
    migration_schema_path = fs.join(schemas_path, "migration-plan.json")
    if fs.exists(migration_schema_path):
        content = fs.read(migration_schema_path)
        schema = json.decode(content)

        # Navigate to op enum: $defs.node.properties.op.enum
        node_def = schema.get("$defs", {}).get("node", {})
        op_prop = node_def.get("properties", {}).get("op", {})

        current_enum = op_prop.get("enum", [])
        if current_enum:
            # migration-plan.json may have a subset of operations (just the ones used in migration)
            # We validate that all its ops exist in the engine, but don't add engine-only ops
            missing_ops = [op for op in current_enum if op not in ops_list]
            if missing_ops:
                warn("  migration-plan.json has invalid operations: " + ", ".join(missing_ops))
                warn("    Valid operations: " + ", ".join(ops_list))
                # Update to only include valid ops
                valid_ops = sorted([op for op in current_enum if op in ops_list])
                op_prop["enum"] = valid_ops
                fs.write(migration_schema_path, json.encode_indent(schema, "  "))
                success("  Wrote " + migration_schema_path)
            else:
                success("  migration-plan.json operations are valid")


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


command(
    name = "devlore-registry.build.knowledge",
    help = "Build knowledge artifacts from devlore-cli source",
    flags = [
        {"name": "source_path", "help": "Path to devlore-cli (default: ../devlore-cli)", "default": ""},
        {"name": "registry_path", "help": "Path to devlore-registry (default: ../devlore-registry)", "default": ""},
        {"name": "domain", "help": "Knowledge domain: all, onboarding, or migration", "default": "all"},
    ],
    run = run,
)
