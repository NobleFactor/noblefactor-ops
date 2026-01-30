# index-packages.star - Generate INDEX.yaml for lore packages
#
# This operation scans the packages/ directory in devlore-registry
# and generates an INDEX.yaml file listing all available packages.

def parse_lifecycle(content):
    """Parse lifecycle.yaml content and extract key fields."""
    data = yaml.decode(content)
    if data == None:
        return None

    return {
        "name": data.get("name", ""),
        "version": data.get("version", ""),
        "description": data.get("description", ""),
        "homepage": data.get("homepage", ""),
        "license": data.get("license", ""),
        "maintainer": data.get("maintainer", ""),
        "platforms": data.get("platforms", []),
        "provides": data.get("provides", []),
        "aliases": data.get("aliases", []),
        "tags": data.get("tags", []),
    }

def scan_packages(packages_dir):
    """Scan packages directory and collect metadata."""
    packages = []

    for entry in fs.list_dir(packages_dir):
        if not entry.is_dir:
            continue

        lifecycle_path = fs.join(entry.path, "lifecycle.yaml")
        if not fs.exists(lifecycle_path):
            warn("Skipping " + entry.name + " (no lifecycle.yaml)")
            continue

        content = fs.read(lifecycle_path)
        pkg = parse_lifecycle(content)
        if pkg == None:
            warn("Skipping " + entry.name + " (invalid lifecycle.yaml)")
            continue

        # Add directory name for reference
        pkg["dir"] = entry.name

        # Check for README
        readme_path = fs.join(entry.path, "README.md")
        pkg["has_readme"] = fs.exists(readme_path)

        # List available platform variants
        variants = []
        for subentry in fs.list_dir(entry.path):
            if subentry.is_dir and not subentry.name.startswith("."):
                variants.append(subentry.name)
        pkg["variants"] = variants

        packages.append(pkg)
        note("Found: " + pkg["name"] + " v" + pkg["version"])

    return packages

def build_index(packages):
    """Build the index structure."""
    # Sort packages by name
    sorted_pkgs = sorted(packages, key=lambda p: p["name"])

    return {
        "version": "1",
        "generated": "nf-ops registry index-packages",
        "count": len(sorted_pkgs),
        "packages": sorted_pkgs,
    }

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
    packages = scan_packages(packages_dir)

    if len(packages) == 0:
        warn("No packages found")
        return

    index = build_index(packages)
    index_content = yaml.encode(index)

    # Determine output path
    if output == "":
        output = fs.join(packages_dir, "INDEX.yaml")

    if dry_run:
        note("Would write: " + output)
        print(index_content)
    else:
        fs.write(output, index_content)
        success("Wrote: " + output)
        note("Indexed " + str(len(packages)) + " package(s)")

# Register the command
command(
    name = "registry.index-packages",
    help = "Generate INDEX.yaml for lore packages in devlore-registry",
    flags = [
        {"name": "path", "help": "Path to devlore-registry", "default": "."},
        {"name": "output", "help": "Output file path (default: packages/INDEX.yaml)", "default": ""},
        {"name": "dry_run", "help": "Show what would be done without writing files", "default": ""},
    ],
    run = run,
)
