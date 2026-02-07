#!/usr/bin/env bash

# SPDX-License-Identifier: MIT
# Copyright Noble Factor. All rights reserved.

#
# Ground Zero Setup - Complete infrastructure setup for a new DevLore project
# Orchestrates Azure Static Web App and GitHub repository configuration
#
# Usage:
#   ./setup-ground-zero.sh --name <project-name> --domain <custom-domain> --repo <owner/repo>
#
# Example:
#   ./setup-ground-zero.sh --name devlore-site --domain devlore.noblefactor.com --repo NobleFactor/devlore.noblefactor.com
#
# Prerequisites:
#   - Azure CLI installed and logged in (az login)
#   - GitHub CLI installed and authenticated (gh auth login)
#   - Admin access to the GitHub repository
#   - Azure subscription with resource creation permissions

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Colors
if [[ -t 1 ]]; then
    RED='\033[0;31m'
    GREEN='\033[0;32m'
    YELLOW='\033[0;33m'
    BLUE='\033[0;34m'
    CYAN='\033[0;36m'
    NC='\033[0m'
else
    RED='' GREEN='' YELLOW='' BLUE='' CYAN='' NC=''
fi

info() { echo -e "${BLUE}info:${NC} $*"; }
success() { echo -e "${GREEN}success:${NC} $*"; }
warn() { echo -e "${YELLOW}warning:${NC} $*"; }
error() {
    echo -e "${RED}error:${NC} $*" >&2
    exit 1
}
phase() { echo -e "\n${CYAN}=== $* ===${NC}\n"; }

# Parse arguments
PROJECT_NAME=""
CUSTOM_DOMAIN=""
GITHUB_REPO=""
DEFAULT_BRANCH="develop"
SKIP_AZURE=false
SKIP_GITHUB=false
DRY_RUN=false

while [[ $# -gt 0 ]]; do
    case $1 in
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
            shift
            ;;
        --skip-github)
            SKIP_GITHUB=true
            shift
            ;;
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        --help | -h)
            cat <<EOF
Ground Zero Setup - Complete DevLore infrastructure provisioning

Usage: $0 --name <project> --domain <domain> --repo <owner/repo> [options]

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
  $0 --name devlore-site --domain devlore.noblefactor.com --repo NobleFactor/devlore.noblefactor.com

EOF
            exit 0
            ;;
        *) error "Unknown option: $1" ;;
    esac
done

# Validate required arguments
[[ -z "$PROJECT_NAME" ]] && error "Missing required --name argument"
[[ -z "$CUSTOM_DOMAIN" ]] && error "Missing required --domain argument"
[[ -z "$GITHUB_REPO" ]] && error "Missing required --repo argument"

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
    warn "DRY RUN MODE - No changes will be made"
    echo ""
fi

# Check prerequisites
phase "Checking Prerequisites"

PREREQ_OK=true

if ! command -v az &>/dev/null; then
    error "Azure CLI not found. Install from https://docs.microsoft.com/en-us/cli/azure/install-azure-cli"
    PREREQ_OK=false
fi

if ! command -v gh &>/dev/null; then
    error "GitHub CLI not found. Install from https://cli.github.com/"
    PREREQ_OK=false
fi

if ! az account show &>/dev/null; then
    warn "Not logged in to Azure. Run 'az login' first."
    PREREQ_OK=false
else
    success "Azure CLI authenticated"
fi

if ! gh auth status &>/dev/null; then
    warn "Not logged in to GitHub. Run 'gh auth login' first."
    PREREQ_OK=false
else
    success "GitHub CLI authenticated"
fi

if ! gh repo view "$GITHUB_REPO" &>/dev/null; then
    warn "Cannot access repository $GITHUB_REPO"
    PREREQ_OK=false
else
    success "Repository $GITHUB_REPO accessible"
fi

[[ "$PREREQ_OK" != "true" ]] && error "Prerequisites not met. Fix the issues above and retry."

# Confirm
echo ""
read -p "Proceed with setup? [y/N] " -n 1 -r
echo
[[ ! $REPLY =~ ^[Yy]$ ]] && exit 1

# Phase 1: Azure Infrastructure
if [[ "$SKIP_AZURE" != "true" ]]; then
    phase "Phase 1: Azure Infrastructure"

    if [[ "$DRY_RUN" == "true" ]]; then
        info "Would run: setup-azure-swa.sh --name $PROJECT_NAME --domain $CUSTOM_DOMAIN"
    else
        # Capture output for secret extraction
        AZURE_OUTPUT=$(mktemp)
        trap 'rm -f "$AZURE_OUTPUT"' EXIT

        "$SCRIPT_DIR/setup-azure-swa.sh" --name "$PROJECT_NAME" --domain "$CUSTOM_DOMAIN" | tee "$AZURE_OUTPUT"

        # Extract secrets from output for GitHub configuration
        AZURE_STATIC_WEB_APPS_API_TOKEN=$(grep -A1 "AZURE_STATIC_WEB_APPS_API_TOKEN:" "$AZURE_OUTPUT" | tail -1 | tr -d '[:space:]')
        export AZURE_STATIC_WEB_APPS_API_TOKEN
        AZURE_CREDENTIALS=$(grep -A1 "AZURE_CREDENTIALS:" "$AZURE_OUTPUT" | tail -1)
        export AZURE_CREDENTIALS
        AAD_CLIENT_ID=$(grep -A1 "AAD_CLIENT_ID:" "$AZURE_OUTPUT" | tail -1 | tr -d '[:space:]')
        export AAD_CLIENT_ID
        AAD_CLIENT_SECRET=$(grep -A1 "AAD_CLIENT_SECRET:" "$AZURE_OUTPUT" | tail -1 | tr -d '[:space:]')
        export AAD_CLIENT_SECRET
    fi
else
    warn "Skipping Azure setup (--skip-azure)"
fi

# Phase 2: GitHub Configuration
if [[ "$SKIP_GITHUB" != "true" ]]; then
    phase "Phase 2: GitHub Configuration"

    if [[ "$DRY_RUN" == "true" ]]; then
        info "Would run: setup-github-repo.sh --repo $GITHUB_REPO --default-branch $DEFAULT_BRANCH"
    else
        "$SCRIPT_DIR/setup-github-repo.sh" --repo "$GITHUB_REPO" --default-branch "$DEFAULT_BRANCH"
    fi
else
    warn "Skipping GitHub setup (--skip-github)"
fi

# Phase 3: Verification
phase "Phase 3: Verification"

if [[ "$DRY_RUN" == "true" ]]; then
    info "Would verify Azure resources and GitHub configuration"
else
    info "Verifying Azure resources..."
    RESOURCE_GROUP="rg-${PROJECT_NAME}"
    if az group show --name "$RESOURCE_GROUP" &>/dev/null; then
        success "Resource group exists: $RESOURCE_GROUP"
    else
        warn "Resource group not found: $RESOURCE_GROUP"
    fi

    if az staticwebapp show --name "$PROJECT_NAME" --resource-group "$RESOURCE_GROUP" &>/dev/null; then
        SWA_HOSTNAME=$(az staticwebapp show --name "$PROJECT_NAME" --resource-group "$RESOURCE_GROUP" --query defaultHostname -o tsv)
        success "Static Web App exists: $PROJECT_NAME ($SWA_HOSTNAME)"
    else
        warn "Static Web App not found: $PROJECT_NAME"
    fi

    info "Verifying GitHub configuration..."
    SECRET_COUNT=$(gh secret list --repo "$GITHUB_REPO" 2>/dev/null | wc -l || echo "0")
    success "GitHub secrets configured: $SECRET_COUNT"

    RULESET_COUNT=$(gh api "repos/${GITHUB_REPO}/rulesets" --jq 'length' 2>/dev/null || echo "0")
    success "GitHub rulesets configured: $RULESET_COUNT"
fi

# Summary
phase "Setup Complete"

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
