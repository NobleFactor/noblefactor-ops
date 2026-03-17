# SPDX-License-Identifier: MIT
# Copyright Noble Factor. All rights reserved.

# receivers.star — Consistent receiver types
#
# Checks that all methods on a type use the same receiver kind (pointer or value).
# Exception: encoding interface methods (UnmarshalJSON, etc.) that require pointer receivers.

# Methods that are allowed to differ from the type's dominant receiver kind.
ENCODING_EXCEPTIONS = [
    "UnmarshalJSON",
    "UnmarshalXML",
    "UnmarshalYAML",
    "UnmarshalText",
    "UnmarshalBinary",
    "Scan",  # database/sql.Scanner
]

def check(ctx):
    """Check for mixed pointer/value receivers on the same type."""
    methods = goast.methods(path=ctx["path"])
    violations = []

    # Group methods by base type name (strip leading *).
    by_type = {}
    for m in methods:
        base = m.receiver_type
        if base.startswith("*"):
            base = base[1:]
        if base not in by_type:
            by_type[base] = []
        by_type[base].append(m)

    for type_name, type_methods in by_type.items():
        pointer_count = 0
        value_count = 0
        for m in type_methods:
            if m.name in ENCODING_EXCEPTIONS:
                continue
            if m.receiver_type.startswith("*"):
                pointer_count += 1
            else:
                value_count += 1

        # If both pointer and value receivers exist, report the minority.
        if pointer_count > 0 and value_count > 0:
            dominant = "pointer" if pointer_count >= value_count else "value"
            minority = "value" if dominant == "pointer" else "pointer"
            for m in type_methods:
                if m.name in ENCODING_EXCEPTIONS:
                    continue
                is_pointer = m.receiver_type.startswith("*")
                if (dominant == "pointer" and not is_pointer) or (dominant == "value" and is_pointer):
                    violations.append({
                        "line": m.line,
                        "message": type_name + "." + m.name + " uses " + minority + " receiver (type uses " + dominant + ")",
                    })

    return violations

def fix(ctx):
    """No auto-fix — changing receiver types can break interface satisfaction."""
    return None
