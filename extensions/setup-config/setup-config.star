# SPDX-License-Identifier: MIT
# Copyright Noble Factor. All rights reserved.

# setup-config.star - Initialize configuration
#
# Initialize star.yaml and sync tool configurations.

def run(ctx):
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
