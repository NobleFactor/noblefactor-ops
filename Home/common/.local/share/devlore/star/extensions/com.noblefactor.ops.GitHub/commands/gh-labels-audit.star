# SPDX-License-Identifier: Apache-2.0
# Copyright Noble Factor. All rights reserved.

"""Which configured repositories lack a kind, Epic:Ops:Process, or a thread label; and which carry one that has drifted."""

# gh-labels-audit.star

load("commands/scheme.star", "label_audit_lines", "label_faults", "label_inventory", "resolve_repos")

def run(_command, ctx):
    """Audit the label set every participating repository must carry; rows by default, a document with --markdown."""
    repos = resolve_repos(ctx.args.get("repo", ""), ctx.args.get("directory", ""))
    rows = label_faults(label_inventory(repos), repos)
    if ctx.args.get("markdown", False):
        return "\n".join(label_audit_lines(rows, repos))
    return rows
