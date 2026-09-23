# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2026 Noble Factor. All rights reserved.
#
# The organization's PSScriptAnalyzer rule set. One file serves three readers:
#
#   CI          .github/scripts/Test-PowerShell.ps1 passes this path with -Settings
#   a machine   writ deploys it to ~/.config/PSScriptAnalyzer/PSScriptAnalyzerSettings.psd1
#   an editor   VS Code's powershell.scriptAnalysis.settingsPath names the deployed copy
#
# The analyzer finds a settings file on its own only in the directory it is analyzing, so every reader names it.
# Ruled 2026-09-22 (#218): the rule set is organization policy and lives in the base layer, not in a consumer.

@{
    # Information and above, ruled 2026-09-22. It is the level the Windows profiles were cleaned to
    # (David-Noble-at-work/personal#199), and the strictest of the three: Information carries the layout and
    # correctness advice -- trailing whitespace, OutputType, unused parameters -- that the style guide already asks for.
    Severity = @('Error', 'Warning', 'Information')

    # Every built-in rule applies. An exclusion goes here with the reason it earns, never silently; a single file that
    # must break a rule uses a [Diagnostics.CodeAnalysis.SuppressMessageAttribute] with its own justification instead,
    # so the exception is read where the code is.
    ExcludeRules = @()
}
