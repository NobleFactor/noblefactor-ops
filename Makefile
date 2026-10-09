# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2025 Noble Factor. All rights reserved.

# The gates CI runs, one target each, and check, which runs them all and then the tests, so a local run and CI are one
# command (#268). CI installs the tools; locally they come from the package manager. GNU make 3.82 or later, for
# .ONESHELL.

SHELL := bash
.SHELLFLAGS := -o errexit -o nounset -o pipefail -c
.ONESHELL:
.DEFAULT_GOAL := help

.PHONY: check frontmatter help powershell shell-lint spelling starlark test

help: ## Show the targets
	awk 'BEGIN { FS = ":.*## " } /^[a-z-]+:.*## / { printf "  %-12s %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

check: frontmatter spelling shell-lint powershell starlark test ## Run every gate CI runs, then the tests

# Enforces docs/documentation-standards.md, which this repository defines for the whole organization. Two document
# families, one rule: a document declaring `type` is a catalog document and takes decision statuses; one without is a
# working document and takes the lifecycle statuses from docs/plans/TEMPLATE.md.
frontmatter: ## Check every document's frontmatter
	./.github/scripts/Test-Frontmatter.sh

# Four checks per tracked .ps1, in .github/scripts/Test-PowerShell.ps1: the shebang and no BOM, the parser,
# [CmdletBinding()] with param() on every function, and PSScriptAnalyzer with the organization's settings
# (Home/common/.config/PSScriptAnalyzer).
powershell: ## Check every PowerShell script
	pwsh -NoProfile -File ./.github/scripts/Test-PowerShell.ps1

# The same gate the personal repository runs, so a script valid there is valid here: shfmt for form, shellcheck for
# substance, and shellcheck follows Declare-BashScript through -P (.github/scripts/shell-lint.sh).
shell-lint: ## Format-check and lint every shell script
	./.github/scripts/shell-lint.sh

# Prose is the product here. codespell has a low false-positive rate; new exceptions go in .github/codespell-ignore, one
# word per line. --builtin carries codespell's own default, clear and rare, plus en-GB_to_en-US (#234): naming the
# locale dictionary alone would drop the other two. The one skipped document is #234's own plan. Its subject is the
# difference between the two spellings, so it has to contain both, and a sweep over it turns its rename table into a
# tautology. That is one file named here, not a word exempted repository-wide.
spelling: ## Check the prose
	codespell --builtin clear,rare,en-GB_to_en-US --ignore-words .github/codespell-ignore \
		--skip './.git,*.svg,*.wasm,*.lock,./docs/plans/fix/234-prose-is-not-checked-for.md'

# buildifier is Bazel's formatter and linter for the language and knows nothing of devlore's provider surface; it
# catches formatting, unused variables, confusing names and missing docstrings, and passes a well-formed call to a
# method that does not exist. The resolution checker that would catch that is a Go built-in (devlore-cli#721); this is
# the interim, ruled 2026-09-05. The two disabled warnings are buildifier's Google-docstring completeness rules;
# one-line docstrings are the house style.
starlark: ## Format-check and lint every Starlark file
	find . \( -path ./.git -o -path ./build \) -prune -o -name '*.star' -print0 |
		xargs --null --no-run-if-empty buildifier -mode=check -lint=warn \
			-warnings=-function-docstring-args,-function-docstring-return

test: ## Run every test under tests/
	for test in tests/*; do
		"./$${test}"
	done
