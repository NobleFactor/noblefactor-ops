# build-signatures.star - Generate signatures.yaml from lifecycle.yaml files
#
# This operation scans all packages/*/lifecycle.yaml files in devlore-registry,
# extracts the signatures field, and builds an inverted index for package detection:
#
#   manager:
#     native_name: lore_package
#
# Usage:
#   star registry.build-signatures --path=/path/to/devlore-registry

def scan_packages(packages_dir):
    """Scan packages directory and collect signatures."""
    # manager -> native_name -> lore_package
    index = {}

    for entry in fs.list_dir(packages_dir):
        if not entry.is_dir:
            continue

        lifecycle_path = fs.join(entry.path, "lifecycle.yaml")
        if not fs.exists(lifecycle_path):
            continue

        content = fs.read(lifecycle_path)
        data = yaml.decode(content)
        if data == None:
            warn("Skipping " + entry.name + " (invalid lifecycle.yaml)")
            continue

        lore_package = data.get("name")
        if not lore_package:
            warn("Skipping " + entry.name + " (no name field)")
            continue

        signatures = data.get("signatures")
        if not signatures:
            continue

        for manager, names in signatures.items():
            if type(names) != "list":
                warn(entry.name + " signatures." + manager + " is not a list")
                continue

            if manager not in index:
                index[manager] = {}

            for name in names:
                if name in index[manager]:
                    existing = index[manager][name]
                    if existing != lore_package:
                        warn(manager + ":" + name + " maps to both " + existing + " and " + lore_package)
                index[manager][name] = lore_package

    return index

def run(ctx):
    """Main entry point."""
    registry = ctx.args.get("path", ".")
    dry_run = ctx.args.get("dry_run", "") == "true"
    output = ctx.args.get("output", "")

    packages_dir = fs.join(registry, "packages")

    if not fs.exists(packages_dir):
        fail("packages/ directory not found at " + packages_dir)
        return

    note("Scanning packages in " + packages_dir)
    index = scan_packages(packages_dir)

    if len(index) == 0:
        warn("No signatures found")
        return

    # Count total mappings
    total = 0
    for manager, names in index.items():
        total = total + len(names)

    # Sort managers for consistent output
    sorted_index = {}
    for manager in sorted(index.keys()):
        sorted_index[manager] = index[manager]

    content = yaml.encode(sorted_index)

    # Determine output path
    if output == "":
        output = fs.join(registry, "signatures.yaml")

    if dry_run:
        note("Would write: " + output)
        print(content)
    else:
        fs.write(output, content)
        success("Wrote: " + output)
        note(str(len(index)) + " managers, " + str(total) + " mappings")

# Register the command
command(
    name = "registry.build-signatures",
    help = "Generate signatures.yaml from lifecycle.yaml files in devlore-registry",
    flags = [
        {"name": "path", "help": "Path to devlore-registry", "default": "."},
        {"name": "output", "help": "Output file path (default: signatures.yaml)", "default": ""},
        {"name": "dry_run", "help": "Show what would be done without writing files", "default": ""},
    ],
    run = run,
)
