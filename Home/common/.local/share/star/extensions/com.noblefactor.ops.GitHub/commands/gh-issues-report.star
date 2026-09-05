# SPDX-License-Identifier: MIT
# Copyright Noble Factor. All rights reserved.

"""The epic / feature / task tree from GitHub issues."""

# gh-issues-report.star
#
# Phase 1 of noblefactor-ops#140: one repository, --by epic, and row-for-row parity with
# devlore-cli's scripts/Get-EpicReport. The result is the rows; --markdown returns the document the
# script prints instead, for the reader and for the parity diff.

load("commands/scheme.star", "audit", "audit_lines", "by_number", "epic_rows", "epic_tag", "exempt_numbers", "fetch_issues", "is_bug", "is_child", "key", "label_space", "line", "parent_feature", "resolve_repos", "table_lines", "tagged", "task_row", "thread_names", "thread_rows", "thread_table_lines")

def _epics(all, epic_filter):
    return by_number([i for i in all if tagged(i, "epic") and (epic_filter == "" or epic_tag(i) == "Epic:" + epic_filter)])

def _header(state, repos):
    out = ["State filter: **" + state + "**."]
    if len(repos) > 1:
        out.append("Repositories: " + ", ".join(repos) + ".")
    return out

def _table_document(all, epics, state, repos):
    out = ["# Epic status\n"] + _header(state, repos)
    if len(epics) == 0:
        out.append("\nNo epics matched.")
        return out
    for e in epics:
        out.append("\n## " + e["title"] + " [#" + str(e["number"]) + "](" + e["url"] + ")\n")
        out.extend(table_lines(epic_rows(e, all)))
    return out

def _tree_document(all, epics, state, repos, a):
    out = ["# Epic report\n"] + _header(state, repos) + ["\n## Classification audit\n"]
    out.extend(audit_lines(a))
    if len(epics) == 0:
        out.append("\nNo epics matched.")
        return out
    out.append("\n_" + str(len(epics)) + " epic(s)._")
    for e in epics:
        tag = epic_tag(e)
        features = by_number([i for i in all if tagged(i, "feature") and epic_tag(i) == tag])
        tasks = [i for i in all if is_child(i) and epic_tag(i) == tag]
        feature_keys = [key(f) for f in features]
        out.append("\n## " + line(e))
        out.append("\n_" + str(len(features)) + " feature(s), " + str(len([t for t in tasks if not is_bug(t)])) + " task(s), " + str(len([t for t in tasks if is_bug(t)])) + " bug(s)._")
        for f in features:
            out.append("\n### " + line(f))
            mine = by_number([t for t in tasks if parent_feature(t) == key(f)])
            if len(mine) == 0:
                out.append("\n_No tasks filed._")
            else:
                out.append("\n| Done | Issue | Title | Kind | Severity | Priority |")
                out.append("|---|---|---|---|---|---|")
                out.extend([task_row(t) for t in mine])
        unfiled = by_number([t for t in tasks if parent_feature(t) == None or parent_feature(t) not in feature_keys])
        if len(unfiled) > 0:
            out.append("\n### Unfiled — no parent feature stated")
            out.append("\n| Done | Issue | Title | Kind | Severity | Priority |")
            out.append("|---|---|---|---|---|---|")
            out.extend([task_row(t) for t in unfiled])
    return out

def _thread_document(threads, state, repos):
    out = ["# Thread status\n"] + _header(state, repos)
    if len(threads) == 0:
        out.append("\nNo threads matched.")
        return out
    for t in threads:
        th = t["thread"]
        if th:
            out.append("\n## " + th["title"] + " [" + th["ref"] + "](" + th["url"] + ")\n")
        else:
            out.append("\n## Thread:" + t["name"] + " (no thread issue)\n")
        out.extend(thread_table_lines(t["rows"]))
        if len(t["faults"]) > 0:
            out.append("")
            for f in t["faults"]:
                out.append("- **fault:** " + f)
    return out

def run(_command, ctx):
    """Render the tree by epic; rows by default, the script's markdown document with --markdown.

    Args:
      _command: the command spec star passes first; unused here.
      ctx: the invocation -- ctx.args carries the flags.
    """
    by = ctx.args.get("by", "epic")
    view = ctx.args.get("view", "table")
    epic_filter = ctx.args.get("epic", "")
    thread_filter = ctx.args.get("thread", "")
    state = ctx.args.get("state", "")
    limit = ctx.args.get("limit", 500)
    directory = ctx.args.get("directory", "")
    repo_flag = ctx.args.get("repo", "")
    markdown = ctx.args.get("markdown", False)

    if by not in ["epic", "thread"]:
        fail("--by must be epic or thread; feature arrives in a later phase of noblefactor-ops#140 (got '" + by + "')")
    if view not in ["tree", "table"]:
        fail("--view must be tree or table (got '" + view + "')")

    # --by thread reports status, and status includes what has closed: the state defaults to all.
    if state == "":
        state = "all" if by == "thread" else "open"
    if state not in ["open", "closed", "all"]:
        fail("--state must be open, closed, or all (got '" + state + "')")

    repos = resolve_repos(repo_flag, directory)
    space = None
    if len(repos) > 1:
        note("repositories: " + ", ".join(repos))
    if by == "thread" or (len(repos) > 1 and epic_filter):
        space = label_space(repos)
    if len(repos) > 1:
        if epic_filter:
            known = sorted({label[len("Epic:"):]: True for r in space for label in space[r] if label.startswith("Epic:")}.keys())
            if epic_filter not in known:
                fail("no Epic:" + epic_filter + " label in " + ", ".join(repos) + "; known epics: " + ", ".join(known))
    all = fetch_issues(state, limit, repos)
    exempt = exempt_numbers()

    if by == "thread":
        names = thread_names(space)
        if thread_filter:
            if thread_filter not in names:
                fail("no Thread:" + thread_filter + " label in " + ", ".join(repos) + "; known threads: " + ", ".join(names))
            names = [thread_filter]
        threads = [thread_rows(n, all) for n in names]
        if markdown:
            return "\n".join(_thread_document(threads, state, repos))
        rows = []
        for t in threads:
            rows.extend(t["rows"])
        return rows

    epics = _epics(all, epic_filter)
    a = audit(all, exempt)

    if markdown:
        lines = _table_document(all, epics, state, repos) if view == "table" else _tree_document(all, epics, state, repos, a)
        return "\n".join(lines)

    rows = []
    for e in epics:
        rows.extend(epic_rows(e, all))
    return rows
