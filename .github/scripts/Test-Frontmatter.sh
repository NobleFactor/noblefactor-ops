#!/usr/bin/env bash

# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2025 Noble Factor. All rights reserved.
#
# Validates the YAML frontmatter this repository's documents carry.
#
# Every tracked markdown file must open with frontmatter and declare `title` and `type`, unless it
# is listed in .github/frontmatter-exempt. Where `status` or `type` is present it must come from the
# vocabulary already in use — a typo in either is invisible to a reader and silently wrong to a
# reader who filters on it.
#
# The exemption list is explicit rather than pattern-based on purpose: a file lacking frontmatter is
# a decision, and decisions belong somewhere a reviewer can see them.

set -euo pipefail

readonly EXEMPT_FILE=".github/frontmatter-exempt"

# TWO DOCUMENT FAMILIES. See docs/documentation-standards.md, which this script enforces.
#
# A document that declares `type:` is a CATALOG document -- an ADR, RFC, or other record whose
# status describes a decision. One that does not is a WORKING document -- a plan or an architecture
# note, whose status describes where the work has got to.
#
# The families are not a repository-by-repository split: docs/guides/ here holds both. The `type`
# field is the discriminator because it is the field that only catalog documents carry, and it
# makes each document self-describing rather than depending on where it happens to sit.
#
# CATALOG_PATHS forces the catalog family regardless, so a tree of ADRs cannot silently
# downgrade itself by omitting `type`. Empty here; the site repository would list its ADR tree.
readonly CATALOG_PATHS=()

readonly REQUIRED_ALWAYS=(title)
readonly REQUIRED_CATALOG=(type)

# Working-document lifecycle: the five statuses docs/plans/TEMPLATE.md lists and
# docs/guides/development-process.md draws (#219). Change them there first, then here.
readonly VALID_STATUS_WORKING=(draft approved active complete abandoned)

# Catalog decision statuses. `status: "Superseded by ADR-019"` is prose in a field otherwise
# treated as an enumeration; the `Superseded by <ref>` form is matched as a shape below, because it
# carries information a bare `Superseded` would lose.
readonly VALID_STATUS_CATALOG=(Draft Proposed Decided Approved Accepted Placeholder Superseded Withdrawn "Research Complete")

readonly VALID_TYPE=(ADR RFC PRD README Overview Pitch Index Demos Strategy Roadmap Reference Guide "Demo Script" Plan Process Brief)

errors=0
checked=0
exempted=0

is_exempt() {
    [[ -f "${EXEMPT_FILE}" ]] || return 1
    grep -qxF "$1" "${EXEMPT_FILE}"
}

in_list() {
    local needle="$1" item
    shift
    for item in "$@"; do
        [[ "${item}" == "${needle}" ]] && return 0
    done
    return 1
}

# frontmatter_of prints the block between the opening and closing `---`, or nothing when a file does
# not open with one.
frontmatter_of() {
    local file="$1"
    head -1 "${file}" | grep -q '^---$' || return 1
    awk 'NR == 1 { next } /^---$/ { exit } { print }' "${file}"
}

field_of() {
    printf '%s\n' "$1" | sed -n "s/^$2:[[:space:]]*//p" | head -1 | tr -d '\r'
}

while IFS= read -r file; do

    if is_exempt "${file}"; then
        exempted=$((exempted + 1))
        continue
    fi

    checked=$((checked + 1))

    if ! block="$(frontmatter_of "${file}")"; then
        echo "ERROR: ${file}: no frontmatter. Add it, or list the file in ${EXEMPT_FILE}."
        errors=$((errors + 1))
        continue
    fi

    # Family: declared `type`, or a path the repository pins to the catalog family.
    type="$(field_of "${block}" type)"
    family=working
    if [[ -n "${type}" ]]; then
        family=catalog
    else
        for glob in ${CATALOG_PATHS[@]+"${CATALOG_PATHS[@]}"}; do
            # shellcheck disable=SC2053 # glob match is the intent
            if [[ "${file}" == ${glob} ]]; then
                family=catalog
                break
            fi
        done
    fi

    required=("${REQUIRED_ALWAYS[@]}")
    if [[ "${family}" == catalog ]]; then
        required+=("${REQUIRED_CATALOG[@]}")
    fi

    for field in "${required[@]}"; do
        if [[ -z "$(field_of "${block}" "${field}")" ]]; then
            echo "ERROR: ${file}: ${family} document is missing '${field}'."
            errors=$((errors + 1))
        fi
    done

    status="$(field_of "${block}" status)"
    status="${status%\"}"
    status="${status#\"}"
    if [[ "${status}" =~ ^Superseded\ by\ .+$ ]]; then
        status="Superseded"
    fi

    if [[ "${family}" == catalog ]]; then
        valid=("${VALID_STATUS_CATALOG[@]}")
    else
        valid=("${VALID_STATUS_WORKING[@]}")
    fi

    if [[ -n "${status}" ]] && ! in_list "${status}" "${valid[@]}"; then
        echo "ERROR: ${file}: ${family} status '${status}' is not one of: ${valid[*]}"
        errors=$((errors + 1))
    fi

    if [[ -n "${type}" ]] && ! in_list "${type}" "${VALID_TYPE[@]}"; then
        echo "ERROR: ${file}: type '${type}' is not one of: ${VALID_TYPE[*]}"
        errors=$((errors + 1))
    fi

done < <(git ls-files '*.md')

echo
echo "frontmatter: ${checked} checked, ${exempted} exempt, ${errors} error(s)"

((errors == 0)) || exit 1
