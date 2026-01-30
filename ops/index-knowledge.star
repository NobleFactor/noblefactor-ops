# index-knowledge.star - Generate index.yaml for knowledge domains
#
# This operation scans the knowledge/ directory in devlore-registry
# and generates index.yaml files for each domain.

def build_file_index(domain_path):
    """Build index of all .md files in a domain."""
    files = []
    for entry in fs.list_dir(domain_path):
        if entry.name == "index.yaml":
            continue
        if entry.is_dir:
            # Recurse into subdirectories
            for subentry in fs.list_dir(entry.path):
                if subentry.name.endswith(".md"):
                    rel_path = entry.name + "/" + subentry.name
                    files.append({
                        "path": rel_path,
                        "name": subentry.name.replace(".md", ""),
                        "category": entry.name,
                    })
        elif entry.name.endswith(".md"):
            files.append({
                "path": entry.name,
                "name": entry.name.replace(".md", ""),
                "category": "root",
            })
    return files

def build_index(domain_name, domain_path):
    """Build the complete index for a domain."""
    files = build_file_index(domain_path)
    return {
        "domain": domain_name,
        "version": "1",
        "files": files,
    }

def run(ctx):
    """Main entry point."""
    registry = ctx.args.get("path", ".")
    dry_run = ctx.args.get("dry_run", "") == "true"

    knowledge_dir = fs.join(registry, "knowledge")

    if not fs.exists(knowledge_dir):
        fail("knowledge/ directory not found at " + knowledge_dir)
        return

    domains_processed = 0

    for entry in fs.list_dir(knowledge_dir):
        if not entry.is_dir:
            continue

        domain_name = entry.name
        domain_path = entry.path

        note("Processing domain: " + domain_name)

        index = build_index(domain_name, domain_path)
        index_content = yaml.encode(index)
        index_path = fs.join(domain_path, "index.yaml")

        if dry_run:
            note("Would write: " + index_path)
            print(index_content)
        else:
            fs.write(index_path, index_content)
            success("Wrote: " + index_path)

        domains_processed = domains_processed + 1

    note("Processed " + str(domains_processed) + " domain(s)")

# Register the command
command(
    name = "registry.index-knowledge",
    help = "Generate index.yaml for knowledge domains in devlore-registry",
    flags = [
        {"name": "path", "help": "Path to devlore-registry", "default": "."},
        {"name": "dry_run", "help": "Show what would be done without writing files", "default": ""},
    ],
    run = run,
)
