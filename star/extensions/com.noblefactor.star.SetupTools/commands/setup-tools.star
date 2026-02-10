# SPDX-License-Identifier: MIT
# Copyright Noble Factor. All rights reserved.

# setup-tools.star - Show development tools status
#
# Display required tools and their installation status.

def run(ctx):
    """Show required tools and their installation status."""
    result = setup.tools()

    note("Development tools for " + result.platform + ":")
    print("")

    for tool in result.tools:
        if tool.installed:
            success(tool.name + ": " + tool.path)
        else:
            error(tool.name + ": not installed")
            note("  " + tool.description)
            note("  Install: " + tool.install_cmd)
            note("  Docs: " + tool.docs_url)
        print("")

    # Summary
    if result.all_installed:
        success("All tools installed")
    else:
        print("")
        note("Install missing tools:")
        for tool in result.tools:
            if not tool.installed:
                print("  " + tool.install_cmd)
        fail(str(result.missing_count) + " tools missing")
