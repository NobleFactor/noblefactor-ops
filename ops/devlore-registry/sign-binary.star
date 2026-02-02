# sign-binary.star - Sign a release binary
#
# This operation signs a release binary (e.g., devlore-cli) with the
# release signing key.
#
# See ADR-040: SSH Key Generation Ceremony for the signing protocol.
#
# Usage:
#   star devlore-registry sign binary --path=dist/devlore-darwin-arm64
#   star devlore-registry sign binary --path=dist/devlore-darwin-arm64 --env=prod

def run(ctx):
    """Sign a release binary."""
    binary_path = ctx.args.get("path", "")
    env = ctx.args.get("env", "dev")

    if not binary_path:
        fail("--path is required")

    fail("Binary signing not yet implemented: " + binary_path)

command(
    name = "devlore-registry.sign.binary",
    help = "Sign a release binary",
    flags = [
        {"name": "path", "help": "Path to binary", "default": ""},
        {"name": "env", "help": "Environment: dev or prod", "default": "dev"},
    ],
    run = run,
)
