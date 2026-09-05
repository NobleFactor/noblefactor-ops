# SPDX-License-Identifier: MIT
# Copyright Noble Factor. All rights reserved.

"""Every open issue the classification scheme cannot place, with the fault."""

# gh-issues-audit.star

load("commands/scheme.star", "audit", "audit_lines", "exempt_numbers", "faults", "fetch_issues", "resolve_repos")

def run(_command, ctx):
    """The classification audit as rows; the script's audit section with --markdown.

    Args:
      _command: the command spec star passes first; unused here.
      ctx: the invocation -- ctx.args carries the flags.
    """
    limit = ctx.args.get("limit", 500)
    directory = ctx.args.get("directory", "")
    repo_flag = ctx.args.get("repo", "")
    markdown = ctx.args.get("markdown", False)

    repos = resolve_repos(repo_flag, directory)
    if len(repos) > 1:
        note("repositories: " + ", ".join(repos))
    all = fetch_issues("open", limit, repos)
    a = audit(all, exempt_numbers())

    if markdown:
        return "\n".join(["## Classification audit\n"] + audit_lines(a))

    return [{"issue": i["number"], "repo": i["repo"], "ref": i["ref"], "url": i["url"], "title": i["title"], "faults": faults(i)} for i in a["bad"]]
