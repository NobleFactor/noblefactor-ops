# SPDX-License-Identifier: MIT
# Copyright Noble Factor. All rights reserved.

# doc-comments.star — Doc comment structure and freshness
#
# Uses the doctaxonomy parser for structural access to doc comments.
# Checks every exported method and unexported helper for:
# 1. Doc comment presence
# 2. Parameters section (present when func has params, items match signature)
# 3. Returns section (present when func has return values)
# 4. Signature synchronization (documented params/returns match code)

def check(ctx):
    """Check doc comment structure and freshness."""
    path = ctx["path"]
    violations = []

    funcs = goast.funcs(path=path)
    methods = goast.methods(path=path)
    all_decls = list(funcs) + list(methods)

    for decl in all_decls:
        c = decl.comment

        # Check doc comment exists.
        if len(c.elements) == 0:
            violations.append({
                "line": decl.line,
                "message": decl.name + ": missing doc comment",
            })
            continue

        # Check Parameters section.
        has_params = len(decl.params) > 0
        ps = c.get_param_section
        if has_params and not ps:
            violations.append({
                "line": decl.line,
                "message": decl.name + ": missing Parameters section",
            })

        if ps:
            sig_params = [p.name for p in decl.params if p.name]
            doc_params = [item.name for item in ps.items]
            for sp in sig_params:
                if sp not in doc_params:
                    violations.append({
                        "line": decl.line,
                        "message": decl.name + ": parameter '" + sp + "' not documented",
                    })
            for dp in doc_params:
                if dp not in sig_params:
                    violations.append({
                        "line": decl.line,
                        "message": decl.name + ": documented parameter '" + dp + "' not in signature",
                    })

        # Check Returns section.
        has_returns = decl.returns != ""
        rs = c.get_return_section
        if has_returns and not rs:
            violations.append({
                "line": decl.line,
                "message": decl.name + ": missing Returns section",
            })

    return violations

def fix(ctx):
    """Fix doc comments by rewrapping to fill width."""
    path = ctx["path"]
    width = ctx["config"]["line_width"]

    content = goast.rewrap_comments(path=path, width=width)
    if content == ctx["content"]:
        return None
    return content
