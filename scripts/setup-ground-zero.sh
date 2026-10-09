#!/usr/bin/env bash

# SPDX-License-Identifier: Apache-2.0
# Copyright Noble Factor. All rights reserved.

# setup-ground-zero.sh - Complete infrastructure setup for a new DevLore project
#
# Orchestrates Azure Static Web App and GitHub repository configuration.
#
# Prerequisites:
#   - Azure CLI installed and logged in (az login)
#   - GitHub CLI installed and authenticated (gh auth login)
#   - Admin access to the GitHub repository
#   - Azure subscription with resource creation permissions

[[ -r "$(dirname "$0")/../Home/common/.local/bin/Declare-BashScript" ]] || {
    printf '%s: cannot find Declare-BashScript in %s\n' "${0##*/}" "$(dirname "$0")/../Home/common/.local/bin" >&2
    exit 72 # EX_OSFILE
}
# shellcheck source=Declare-BashScript
source "$(dirname "$0")/../Home/common/.local/bin/Declare-BashScript" "$0" \
    "help,name:,domain:,repo:,default-branch:,skip-azure,skip-github,dry-run" "h" "$@"
require_nix

###########
# Functions
###########

AZURE_OUTPUT=""

# cleanup removes the file Phase 1 captures the Azure setup's output in, whichever way the script ends.
function cleanup {
    [[ -z "${AZURE_OUTPUT}" ]] || rm -f "${AZURE_OUTPUT}"
}

Set-Traps cleanup

###########
# Arguments
###########

declare -r synopsis="Ground Zero Setup - Complete DevLore infrastructure provisioning

Usage: setup-ground-zero.sh --name <project> --domain <domain> --repo <owner/repo> [options]

Required:
  --name     Project name (used for Azure resource naming)
  --domain   Custom domain (e.g., devlore.noblefactor.com)
  --repo     GitHub repository (owner/repo format)

Options:
  --default-branch   Default branch (default: develop)
  --skip-azure       Skip Azure resource creation
  --skip-github      Skip GitHub configuration
  --dry-run          Show what would be done without making changes

Prerequisites:
  - Azure CLI: az login
  - GitHub CLI: gh auth login
  - Admin access to GitHub repository
  - Azure subscription with Contributor permissions

What this script does:

  Phase 1: Azure Infrastructure
    - Create resource group
    - Create Static Web App (Standard tier)
    - Create Entra ID app for authentication
    - Create service principal for GitHub Actions

  Phase 2: GitHub Configuration
    - Set repository secrets
    - Configure branch protection rulesets
    - Configure merge settings (squash only)

  Phase 3: Verification
    - Test Azure resource access
    - Verify GitHub secrets
    - Output DNS configuration

Example:
  setup-ground-zero.sh --name devlore-site --domain devlore.noblefactor.com --repo NobleFactor/devlore.noblefactor.com"

eval set -- "$script_arguments"

PROJECT_NAME=""
CUSTOM_DOMAIN=""
GITHUB_REPO=""
DEFAULT_BRANCH="develop"
SKIP_AZURE=false
SKIP_GITHUB=false
DRY_RUN=false

while :; do
    case $1 in
        -h | --help)
            usage "$synopsis"
            ;;
        --name)
            PROJECT_NAME="$2"
            shift 2
            ;;
        --domain)
            CUSTOM_DOMAIN="$2"
            shift 2
            ;;
        --repo)
            GITHUB_REPO="$2"
            shift 2
            ;;
        --default-branch)
            DEFAULT_BRANCH="$2"
            shift 2
            ;;
        --skip-azure)
            SKIP_AZURE=true
            shift 1
            ;;
        --skip-github)
            SKIP_GITHUB=true
            shift 1
            ;;
        --dry-run)
            DRY_RUN=true
            shift 1
            ;;
        --)
            shift 1
            break
            ;;
        *)
            error $EX_USAGE "Unrecognized option: $1"
            ;;
    esac
done

(($# == 0)) || error $EX_USAGE "Unexpected argument: $1"
[[ -n "$PROJECT_NAME" ]] || error $EX_USAGE "Missing required --name argument"
[[ -n "$CUSTOM_DOMAIN" ]] || error $EX_USAGE "Missing required --domain argument"
[[ -n "$GITHUB_REPO" ]] || error $EX_USAGE "Missing required --repo argument"

######
# Main
######

# Banner
echo ""
echo "=============================================="
echo "  DevLore Ground Zero Setup"
echo "=============================================="
echo ""
echo "Project:    $PROJECT_NAME"
echo "Domain:     $CUSTOM_DOMAIN"
echo "Repository: $GITHUB_REPO"
echo "Branch:     $DEFAULT_BRANCH"
echo ""

if [[ "$DRY_RUN" == "true" ]]; then
    note "DRY RUN MODE - No changes will be made"
    echo ""
fi

note "Checking Prerequisites"

readonly azure_cli_url="https://docs.microsoft.com/en-us/cli/azure/install-azure-cli"
command -v az >/dev/null 2>&1 || error $EX_UNAVAILABLE "Azure CLI not found. Install from ${azure_cli_url}"
command -v gh >/dev/null 2>&1 || error $EX_UNAVAILABLE "GitHub CLI not found. Install from https://cli.github.com/"

PREREQ_OK=true

if ! az account show &>/dev/null; then
    error 0 "Not logged in to Azure. Run 'az login' first."
    PREREQ_OK=false
else
    success "Azure CLI authenticated"
fi

if ! gh auth status &>/dev/null; then
    error 0 "Not logged in to GitHub. Run 'gh auth login' first."
    PREREQ_OK=false
else
    success "GitHub CLI authenticated"
fi

if ! gh repo view "$GITHUB_REPO" &>/dev/null; then
    error 0 "Cannot access repository $GITHUB_REPO"
    PREREQ_OK=false
else
    success "Repository $GITHUB_REPO accessible"
fi

[[ "$PREREQ_OK" == "true" ]] || error $EX_UNAVAILABLE "Prerequisites not met. Fix the issues above and retry."

# Confirm
echo ""
read -p "Proceed with setup? [y/N] " -n 1 -r
echo
[[ $REPLY =~ ^[Yy]$ ]] || error $EX_TEMPFAIL "Not confirmed; nothing was changed."

# Phase 1: Azure Infrastructure
if [[ "$SKIP_AZURE" != "true" ]]; then
    note "Phase 1: Azure Infrastructure"

    if [[ "$DRY_RUN" == "true" ]]; then
        note "Would run: setup-azure-swa.sh --name $PROJECT_NAME --domain $CUSTOM_DOMAIN"
    else
        # Capture output for secret extraction; cleanup removes the file however the script ends.
        AZURE_OUTPUT=$(mktemp)

        "$script_root/setup-azure-swa.sh" --name "$PROJECT_NAME" --domain "$CUSTOM_DOMAIN" | tee "$AZURE_OUTPUT"

        # Extract secrets from output for GitHub configuration
        AZURE_STATIC_WEB_APPS_API_TOKEN=$(grep -A1 "AZURE_STATIC_WEB_APPS_API_TOKEN:" "$AZURE_OUTPUT" | tail -1 |
            tr -d '[:space:]')
        export AZURE_STATIC_WEB_APPS_API_TOKEN
        AZURE_CREDENTIALS=$(grep -A1 "AZURE_CREDENTIALS:" "$AZURE_OUTPUT" | tail -1)
        export AZURE_CREDENTIALS
        AAD_CLIENT_ID=$(grep -A1 "AAD_CLIENT_ID:" "$AZURE_OUTPUT" | tail -1 | tr -d '[:space:]')
        export AAD_CLIENT_ID
        AAD_CLIENT_SECRET=$(grep -A1 "AAD_CLIENT_SECRET:" "$AZURE_OUTPUT" | tail -1 | tr -d '[:space:]')
        export AAD_CLIENT_SECRET
    fi
else
    note "Skipping Azure setup (--skip-azure)"
fi

# Phase 2: GitHub Configuration
if [[ "$SKIP_GITHUB" != "true" ]]; then
    note "Phase 2: GitHub Configuration"

    if [[ "$DRY_RUN" == "true" ]]; then
        note "Would run: setup-github-repo.sh --repo $GITHUB_REPO --default-branch $DEFAULT_BRANCH"
    else
        "$script_root/setup-github-repo.sh" --repo "$GITHUB_REPO" --default-branch "$DEFAULT_BRANCH"
    fi
else
    note "Skipping GitHub setup (--skip-github)"
fi

# Phase 3: Verification
note "Phase 3: Verification"

if [[ "$DRY_RUN" == "true" ]]; then
    note "Would verify Azure resources and GitHub configuration"
else
    note "Verifying Azure resources..."
    RESOURCE_GROUP="rg-${PROJECT_NAME}"
    if az group show --name "$RESOURCE_GROUP" &>/dev/null; then
        success "Resource group exists: $RESOURCE_GROUP"
    else
        error 0 "Resource group not found: $RESOURCE_GROUP"
    fi

    if az staticwebapp show --name "$PROJECT_NAME" --resource-group "$RESOURCE_GROUP" &>/dev/null; then
        SWA_HOSTNAME=$(az staticwebapp show --name "$PROJECT_NAME" --resource-group "$RESOURCE_GROUP" \
            --query defaultHostname -o tsv)
        success "Static Web App exists: $PROJECT_NAME ($SWA_HOSTNAME)"
    else
        error 0 "Static Web App not found: $PROJECT_NAME"
    fi

    note "Verifying GitHub configuration..."
    SECRET_COUNT=$(gh secret list --repo "$GITHUB_REPO" 2>/dev/null | wc -l) || SECRET_COUNT=0
    success "GitHub secrets configured: $SECRET_COUNT"

    RULESET_COUNT=$(gh api "repos/${GITHUB_REPO}/rulesets" --jq 'length' 2>/dev/null || echo "0")
    success "GitHub rulesets configured: $RULESET_COUNT"
fi

# Summary
note "Setup Complete"

cat <<EOF
Ground Zero setup completed for $PROJECT_NAME.

Azure Resources:
  Resource Group:    rg-${PROJECT_NAME}
  Static Web App:    ${PROJECT_NAME}
  Entra ID App:      ${PROJECT_NAME}-auth

GitHub Configuration:
  Repository:        ${GITHUB_REPO}
  Default Branch:    ${DEFAULT_BRANCH}
  Branch Protection: main, develop, release/*

Next Steps:
  1. Configure DNS (CNAME record pointing to SWA hostname)
  2. Register custom domain:
     az staticwebapp hostname set --name ${PROJECT_NAME} --resource-group rg-${PROJECT_NAME} --hostname ${CUSTOM_DOMAIN}
  3. Deploy your site via git push to trigger GitHub Actions
  4. Invite stakeholders in Entra ID app

Documentation:
  https://devlore.noblefactor.com

EOF
