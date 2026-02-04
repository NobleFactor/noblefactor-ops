# setup.star - Repository setup commands
#
# Ensures a repository is ready for development with all required tools,
# hooks, and configuration in place.
#
# Usage:
#   star setup              # Run all setup tasks
#   star setup tools        # Show required tools and install status
#   star setup hooks        # Install pre-commit hooks
#   star setup config       # Initialize star.yaml and sync tool configs

def run_setup(ctx):
    """Run all setup tasks."""
    note("Setting up repository...")

    # Check tools first
    tools_result = setup.tools()
    if tools_result.missing_count > 0:
        warn(str(tools_result.missing_count) + " tools missing (run 'star setup tools' for details)")
    else:
        success("All " + str(len(tools_result.tools)) + " tools installed")

    # Initialize config
    config_result = setup.init_config()
    if config_result.star_yaml_created:
        success("Created star.yaml")
    for cfg in config_result.configs_synced:
        success("Synced " + cfg)

    # Install pre-commit hooks
    hooks_result = setup.precommit_install()
    if hooks_result.success:
        if hooks_result.already_installed:
            note("Pre-commit hooks already installed")
        else:
            success("Installed pre-commit hooks")
    else:
        if "No .pre-commit-config.yaml" in hooks_result.message:
            note("No .pre-commit-config.yaml found (skipping hooks)")
        else:
            warn(hooks_result.message)

    # Final status
    if tools_result.missing_count > 0:
        warn("Setup complete with warnings - install missing tools")
    else:
        success("Repository setup complete")

def run_tools(ctx):
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

def run_hooks(ctx):
    """Install pre-commit hooks."""
    # First check status
    check = setup.precommit_check()

    if not check.config_exists:
        note("No .pre-commit-config.yaml found in this repository")
        return

    if not check.precommit_available:
        fail("pre-commit not installed. Run: star setup tools")

    # Install hooks
    result = setup.precommit_install()

    if result.success:
        if result.already_installed:
            success("Pre-commit hooks already installed")
        else:
            success("Pre-commit hooks installed")
    else:
        fail(result.message)

def run_config(ctx):
    """Initialize star.yaml and sync tool configurations."""
    result = setup.init_config()

    if result.star_yaml_created:
        success("Created " + result.star_yaml_path)
    else:
        note(result.star_yaml_path + " already exists")

    if len(result.configs_synced) > 0:
        for cfg in result.configs_synced:
            success("Synced " + cfg)
    else:
        note("Tool configs already up to date")

def run_check(ctx):
    """Check setup status without making changes."""
    issues = []

    # Check tools
    tools_result = setup.tools()
    if tools_result.missing_count > 0:
        for tool in tools_result.tools:
            if not tool.installed:
                issues.append("Missing tool: " + tool.name)

    # Check pre-commit
    hooks = setup.precommit_check()
    if hooks.config_exists and not hooks.installed:
        issues.append("Pre-commit hooks not installed")

    # Check star.yaml
    # (config.load will check this)

    # Report
    if len(issues) == 0:
        success("Repository setup is complete")
    else:
        for issue in issues:
            error(issue)
        fail(str(len(issues)) + " setup issues found")

# Register commands
command(
    name = "setup",
    help = "Run all setup tasks (tools check, config init, hooks install)",
    flags = [],
    run = run_setup,
)

command(
    name = "setup.tools",
    help = "Show required development tools and installation status",
    flags = [],
    run = run_tools,
)

command(
    name = "setup.hooks",
    help = "Install pre-commit hooks",
    flags = [],
    run = run_hooks,
)

command(
    name = "setup.config",
    help = "Initialize star.yaml and sync tool configurations",
    flags = [],
    run = run_config,
)

command(
    name = "setup.check",
    help = "Check setup status without making changes",
    flags = [],
    run = run_check,
)
