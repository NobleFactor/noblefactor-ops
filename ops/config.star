# SPDX-License-Identifier: MIT
# Copyright Noble Factor. All rights reserved.

# config.star - Configuration management commands
#
# Manages star.yaml configuration with hierarchy support:
#   1. ./star.yaml (project - highest priority)
#   2. ${XDG_CONFIG_HOME}/star/star.yaml (user defaults)
#   3. Built-in defaults
#
# Usage:
#   star config show              # Show merged configuration
#   star config sync              # Sync tool configs from star.yaml

def run_show(ctx):
    """Show the merged configuration and its sources."""
    result = config.show()

    note("Configuration sources:")
    for source in result.sources:
        if source.exists:
            success("  " + source.path)
        else:
            note("  " + source.path + " (not found)")

    print("")
    note("Merged configuration:")
    _print_config(result.config, 0)

def _print_config(value, indent):
    """Recursively print configuration with indentation."""
    prefix = "  " * indent

    if type(value) == "dict":
        for key in value:
            v = value[key]
            if type(v) == "dict":
                print(prefix + key + ":")
                _print_config(v, indent + 1)
            elif type(v) == "list":
                print(prefix + key + ":")
                for item in v:
                    print(prefix + "  - " + str(item))
            else:
                print(prefix + key + ": " + str(v))
    else:
        print(prefix + str(value))

def run_sync(ctx):
    """Sync tool-specific config files from star.yaml."""
    result = config.sync()

    if result.files_generated == 0:
        note("No tool configs to sync (no config sections in star.yaml)")
        return

    if result.golangci_lint:
        success("Generated " + result.golangci_lint)
    if result.markdown_lint:
        success("Generated " + result.markdown_lint)

    success("Synced " + str(result.files_generated) + " config file(s)")

# Register commands
command(
    name = "config.show",
    help = "Show merged configuration from star.yaml hierarchy",
    flags = [],
    run = run_show,
)

command(
    name = "config.sync",
    help = "Sync tool configs (.golangci.yaml, etc.) from star.yaml",
    flags = [],
    run = run_sync,
)
