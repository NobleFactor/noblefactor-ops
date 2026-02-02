# SPDX-License-Identifier: MIT
# Copyright (c) 2025-2026 Noble Factor. All rights reserved.
#
# modelfile.star - Generate Ollama Modelfiles from knowledge domains
#
# This operation assembles knowledge assets (prompts, signatures, schemas,
# examples) into an Ollama Modelfile with a comprehensive SYSTEM prompt.
#
# Commands:
#   modelfile.generate - Generate Modelfile content
#   modelfile.create   - Generate and create model in Ollama
#   modelfile.list     - List available knowledge domains
#
# Usage:
#   nf-ops modelfile.generate --domain migration --model qwen3:8b
#   nf-ops modelfile.create --domain migration --name devlore-migrate
#   nf-ops modelfile.list --path /path/to/devlore-registry

# Default base model for Modelfiles
DEFAULT_MODEL = "qwen3:8b"

# Asset types to include in SYSTEM prompt (in order)
SYSTEM_ASSET_ORDER = [
    ("prompts", "MAIN PROMPT", ".txt"),
    ("concepts", "CONCEPTS", ".yaml"),
    ("signatures", "DETECTION SIGNATURES", ".yaml"),
    ("transforms", "TRANSFORM RULES", ".yaml"),
    ("examples", "FEW-SHOT EXAMPLES", ".yaml"),
    ("schemas", "OUTPUT SCHEMA", ".json"),
]

# =============================================================================
# MODELFILE GENERATION
# =============================================================================

def build_modelfile(knowledge_path, domain, model):
    """Assemble Modelfile from knowledge assets."""
    lines = []

    # Header
    lines.append("# Generated Modelfile for domain: " + domain)
    lines.append("# Generated: " + _timestamp())
    lines.append("# Source: " + knowledge_path)
    lines.append("")
    lines.append("FROM " + model)
    lines.append("")
    lines.append('SYSTEM """')

    # Include each asset type in order
    for asset_type, section_name, extension in SYSTEM_ASSET_ORDER:
        asset_dir = fs.join(knowledge_path, asset_type)
        if not fs.is_dir(asset_dir):
            continue

        section_content = _build_section(asset_dir, section_name, extension)
        if section_content:
            lines.append(section_content)

    # Close SYSTEM prompt
    lines.append('"""')
    lines.append("")

    # Parameters optimized for structured output
    lines.append("PARAMETER temperature 0.1")
    lines.append("PARAMETER num_ctx 32768")
    lines.append("")

    return "\n".join(lines)


def _build_section(asset_dir, section_name, extension):
    """Build a section of the SYSTEM prompt from asset files."""
    files = _list_asset_files(asset_dir, extension)
    if not files:
        return ""

    lines = []
    lines.append("# === " + section_name + " ===")
    lines.append("")

    for filename in files:
        filepath = fs.join(asset_dir, filename)
        content = fs.read(filepath)

        # Skip baseline/generated files in examples
        if filename.startswith("baseline-"):
            continue

        # Determine how to format based on extension
        name = filename
        if extension:
            name = filename.replace(extension, "")

        if extension == ".txt":
            # Plain text - include directly
            lines.append(content)
        elif extension == ".yaml" or extension == ".json":
            # Structured data - wrap in code fence
            lines.append("## " + name)
            lines.append("")
            lines.append("```" + extension.replace(".", ""))
            lines.append(content.strip())
            lines.append("```")
            lines.append("")

    return "\n".join(lines)


def _list_asset_files(dir_path, extension):
    """List files in directory matching extension."""
    files = []
    if not fs.exists(dir_path):
        return files

    for entry in fs.list_dir(dir_path):
        if entry.is_dir:
            continue
        if entry.name.startswith("."):
            continue
        if extension and not entry.name.endswith(extension):
            continue
        files.append(entry.name)

    return sorted(files)


def _timestamp():
    """Get current UTC timestamp placeholder."""
    # Starlark runtime doesn't have time module or shell.run
    # Timestamp will be filled in at runtime if needed
    return "(generated)"


def _find_registry(explicit_path):
    """Find devlore-registry path."""
    if explicit_path:
        return explicit_path

    # Try sibling directory
    sibling = fs.join("..", "devlore-registry")
    if fs.is_dir(sibling):
        return sibling

    # Try current directory
    if fs.is_dir("knowledge"):
        return "."

    return ""


# =============================================================================
# COMMANDS
# =============================================================================

def run_generate(ctx):
    """Generate Modelfile from knowledge domain."""
    domain = ctx.args.get("domain", "migration")
    model = ctx.args.get("model", DEFAULT_MODEL)
    output = ctx.args.get("output", "")
    registry_path = ctx.args.get("path", "")

    registry = _find_registry(registry_path)
    if not registry:
        fail("Cannot find devlore-registry. Use --path to specify location.")

    knowledge_path = fs.join(registry, "knowledge", domain)
    if not fs.is_dir(knowledge_path):
        fail("Knowledge domain not found: " + domain)

    content = build_modelfile(knowledge_path, domain, model)

    if output:
        fs.write(output, content)
        success("Generated: " + output)

        # Show stats
        lines = content.count("\n")
        note("  Model: " + model)
        note("  Domain: " + domain)
        note("  Lines: " + str(lines))
    else:
        print(content)


def run_create(ctx):
    """Generate Modelfile and provide create instructions."""
    domain = ctx.args.get("domain", "migration")
    model = ctx.args.get("model", DEFAULT_MODEL)
    name = ctx.args.get("name", "")
    registry_path = ctx.args.get("path", "")

    if not name:
        name = "devlore-" + domain

    registry = _find_registry(registry_path)
    if not registry:
        fail("Cannot find devlore-registry. Use --path to specify location.")

    knowledge_path = fs.join(registry, "knowledge", domain)
    if not fs.is_dir(knowledge_path):
        fail("Knowledge domain not found: " + domain)

    # Generate Modelfile content
    content = build_modelfile(knowledge_path, domain, model)

    # Write Modelfile
    modelfile_path = name + ".Modelfile"
    fs.write(modelfile_path, content)
    success("Wrote Modelfile: " + modelfile_path)

    # Show stats
    lines = content.count("\n")
    note("  Base model: " + model)
    note("  Domain: " + domain)
    note("  Lines: " + str(lines))

    # Provide instructions
    print("")
    note("To create the Ollama model, run:")
    print("  ollama create " + name + " -f " + modelfile_path)
    print("")
    note("To use the model:")
    print("  ollama run " + name)


def run_list(ctx):
    """List available knowledge domains."""
    registry_path = ctx.args.get("path", "")

    registry = _find_registry(registry_path)
    if not registry:
        fail("Cannot find devlore-registry. Use --path to specify location.")

    knowledge_dir = fs.join(registry, "knowledge")
    if not fs.is_dir(knowledge_dir):
        fail("knowledge/ directory not found at " + knowledge_dir)

    note("Available domains in " + registry + ":")
    print("")

    for entry in fs.list_dir(knowledge_dir):
        if not entry.is_dir:
            continue

        domain_path = entry.path

        # Count assets
        asset_counts = []
        for asset_type, _, ext in SYSTEM_ASSET_ORDER:
            asset_dir = fs.join(domain_path, asset_type)
            files = _list_asset_files(asset_dir, ext)
            if files:
                asset_counts.append(asset_type + ":" + str(len(files)))

        if asset_counts:
            print("  " + entry.name)
            print("    " + ", ".join(asset_counts))
        else:
            print("  " + entry.name + " (empty)")

    print("")


# =============================================================================
# COMMAND REGISTRATION
# =============================================================================

command(
    name = "modelfile.generate",
    help = "Generate Ollama Modelfile from knowledge domain",
    flags = [
        {"name": "domain", "help": "Knowledge domain (migration, onboarding)", "default": "migration"},
        {"name": "model", "help": "Base model for Modelfile", "default": DEFAULT_MODEL},
        {"name": "output", "help": "Output file (default: stdout)", "default": ""},
        {"name": "path", "help": "Path to devlore-registry", "default": ""},
    ],
    run = run_generate,
)

command(
    name = "modelfile.create",
    help = "Generate Modelfile and show create instructions",
    flags = [
        {"name": "domain", "help": "Knowledge domain (migration, onboarding)", "default": "migration"},
        {"name": "model", "help": "Base model for Modelfile", "default": DEFAULT_MODEL},
        {"name": "name", "help": "Model name (default: devlore-<domain>)", "default": ""},
        {"name": "path", "help": "Path to devlore-registry", "default": ""},
    ],
    run = run_create,
)

command(
    name = "modelfile.list",
    help = "List available knowledge domains for Modelfile generation",
    flags = [
        {"name": "path", "help": "Path to devlore-registry", "default": ""},
    ],
    run = run_list,
)
