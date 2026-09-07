#!/usr/bin/env bash

# SPDX-License-Identifier: Apache-2.0
# Copyright Noble Factor. All rights reserved.

#
# GitHub Repository Setup Script
# Configures branch protection, secrets, and repository settings
#
# Usage:
#   ./setup-github-repo.sh --repo <owner/repo> [--default-branch <branch>]
#
# Example:
#   ./setup-github-repo.sh --repo NobleFactor/devlore.noblefactor.com --default-branch develop
#
# Prerequisites:
#   - GitHub CLI installed and authenticated (gh auth login)
#   - Admin access to the repository
#
# Environment variables for secrets (optional):
#   AZURE_STATIC_WEB_APPS_API_TOKEN  - SWA deployment token
#   AZURE_CREDENTIALS                - Service principal JSON
#   SITE_DEPLOY_TOKEN                - Cross-repo sync token

set -euo pipefail

# Colors
if [[ -t 1 ]]; then
    RED='\033[0;31m'
    GREEN='\033[0;32m'
    YELLOW='\033[0;33m'
    BLUE='\033[0;34m'
    NC='\033[0m'
else
    RED='' GREEN='' YELLOW='' BLUE='' NC=''
fi

info() { echo -e "${BLUE}info:${NC} $*"; }
success() { echo -e "${GREEN}success:${NC} $*"; }
warn() { echo -e "${YELLOW}warning:${NC} $*"; }
error() {
    echo -e "${RED}error:${NC} $*" >&2
    exit 1
}

# Parse arguments
REPO=""
DEFAULT_BRANCH="develop"
SKIP_SECRETS=false
SKIP_PROTECTION=false

while [[ $# -gt 0 ]]; do
    case $1 in
        --repo)
            REPO="$2"
            shift 2
            ;;
        --default-branch)
            DEFAULT_BRANCH="$2"
            shift 2
            ;;
        --skip-secrets)
            SKIP_SECRETS=true
            shift
            ;;
        --skip-protection)
            SKIP_PROTECTION=true
            shift
            ;;
        --help | -h)
            echo "Usage: $0 --repo <owner/repo> [options]"
            echo ""
            echo "Options:"
            echo "  --repo             Repository (owner/repo format)"
            echo "  --default-branch   Default branch (default: develop)"
            echo "  --skip-secrets     Skip setting secrets"
            echo "  --skip-protection  Skip branch protection rules"
            echo ""
            echo "Environment variables for secrets:"
            echo "  AZURE_STATIC_WEB_APPS_API_TOKEN"
            echo "  AZURE_CREDENTIALS"
            echo "  SITE_DEPLOY_TOKEN"
            exit 0
            ;;
        *) error "Unknown option: $1" ;;
    esac
done

[[ -z "$REPO" ]] && error "Missing required --repo argument"

# Check prerequisites
info "Checking prerequisites..."

if ! command -v gh &>/dev/null; then
    error "GitHub CLI not found. Install from https://cli.github.com/"
fi

if ! gh auth status &>/dev/null; then
    error "Not logged in to GitHub. Run 'gh auth login' first."
fi

# Verify repo exists and we have admin access
if ! gh repo view "$REPO" &>/dev/null; then
    error "Repository $REPO not found or no access"
fi

info "Repository: $REPO"
info "Default branch: $DEFAULT_BRANCH"
echo ""

# Phase 1: Set secrets
if [[ "$SKIP_SECRETS" != "true" ]]; then
    info "Setting repository secrets..."

    if [[ -n "${AZURE_STATIC_WEB_APPS_API_TOKEN:-}" ]]; then
        echo "$AZURE_STATIC_WEB_APPS_API_TOKEN" | gh secret set AZURE_STATIC_WEB_APPS_API_TOKEN --repo "$REPO"
        success "Set AZURE_STATIC_WEB_APPS_API_TOKEN"
    else
        warn "AZURE_STATIC_WEB_APPS_API_TOKEN not set, skipping"
    fi

    if [[ -n "${AZURE_CREDENTIALS:-}" ]]; then
        echo "$AZURE_CREDENTIALS" | gh secret set AZURE_CREDENTIALS --repo "$REPO"
        success "Set AZURE_CREDENTIALS"
    else
        warn "AZURE_CREDENTIALS not set, skipping"
    fi

    if [[ -n "${SITE_DEPLOY_TOKEN:-}" ]]; then
        echo "$SITE_DEPLOY_TOKEN" | gh secret set SITE_DEPLOY_TOKEN --repo "$REPO"
        success "Set SITE_DEPLOY_TOKEN"
    else
        warn "SITE_DEPLOY_TOKEN not set, skipping"
    fi

    echo ""
fi

# Phase 2: Configure branch protection via repository rulesets
# Per https://docs.github.com/en/rest/repos/rules
if [[ "$SKIP_PROTECTION" != "true" ]]; then
    info "Configuring branch protection rulesets..."

    # Check if ruleset already exists
    EXISTING_RULESET=$(gh api "repos/${REPO}/rulesets" --jq '.[] | select(.name == "Branch Protection") | .id' 2>/dev/null || true)

    if [[ -n "$EXISTING_RULESET" ]]; then
        warn "Ruleset 'Branch Protection' already exists (id: $EXISTING_RULESET)"
        info "Updating existing ruleset..."

        gh api --method PUT "repos/${REPO}/rulesets/${EXISTING_RULESET}" \
            --input - <<EOF
{
    "name": "Branch Protection",
    "target": "branch",
    "enforcement": "active",
    "conditions": {
        "ref_name": {
            "include": ["~DEFAULT_BRANCH", "refs/heads/main", "refs/heads/release/*"],
            "exclude": []
        }
    },
    "rules": [
        {
            "type": "pull_request",
            "parameters": {
                "required_approving_review_count": 1,
                "dismiss_stale_reviews_on_push": true,
                "require_code_owner_review": false,
                "require_last_push_approval": false,
                "required_review_thread_resolution": false
            }
        },
        {
            "type": "required_linear_history"
        },
        {
            "type": "non_fast_forward"
        }
    ],
    "bypass_actors": [
        {
            "actor_id": 5,
            "actor_type": "RepositoryRole",
            "bypass_mode": "pull_request"
        }
    ]
}
EOF
        success "Updated branch protection ruleset"
    else
        info "Creating new ruleset..."

        gh api --method POST "repos/${REPO}/rulesets" \
            --input - <<EOF
{
    "name": "Branch Protection",
    "target": "branch",
    "enforcement": "active",
    "conditions": {
        "ref_name": {
            "include": ["~DEFAULT_BRANCH", "refs/heads/main", "refs/heads/release/*"],
            "exclude": []
        }
    },
    "rules": [
        {
            "type": "pull_request",
            "parameters": {
                "required_approving_review_count": 1,
                "dismiss_stale_reviews_on_push": true,
                "require_code_owner_review": false,
                "require_last_push_approval": false,
                "required_review_thread_resolution": false
            }
        },
        {
            "type": "required_linear_history"
        },
        {
            "type": "non_fast_forward"
        }
    ],
    "bypass_actors": [
        {
            "actor_id": 5,
            "actor_type": "RepositoryRole",
            "bypass_mode": "pull_request"
        }
    ]
}
EOF
        success "Created branch protection ruleset"
    fi

    echo ""
fi

# Phase 3: Configure repository settings
info "Configuring repository settings..."

# Enable squash merging only, disable merge commits and rebase
gh api --method PATCH "repos/${REPO}" \
    --field allow_squash_merge=true \
    --field allow_merge_commit=false \
    --field allow_rebase_merge=false \
    --field delete_branch_on_merge=true \
    --field allow_auto_merge=false \
    --silent

success "Configured merge settings (squash only, delete branch on merge)"

# Set default branch if different
CURRENT_DEFAULT=$(gh repo view "$REPO" --json defaultBranchRef --jq '.defaultBranchRef.name')
if [[ "$CURRENT_DEFAULT" != "$DEFAULT_BRANCH" ]]; then
    info "Setting default branch to $DEFAULT_BRANCH..."
    gh api --method PATCH "repos/${REPO}" \
        --field default_branch="$DEFAULT_BRANCH" \
        --silent
    success "Set default branch to $DEFAULT_BRANCH"
fi

echo ""
echo "=============================================="
echo "  GitHub Repository Setup Complete"
echo "=============================================="
echo ""
echo "Repository:      $REPO"
echo "Default Branch:  $DEFAULT_BRANCH"
echo ""
echo "Branch Protection:"
echo "  - PRs required for: main, develop, release/*"
echo "  - 1 approval required"
echo "  - Stale reviews dismissed"
echo "  - Admins bypass: PR merge only (no direct push)"
echo ""
echo "Merge Settings:"
echo "  - Squash merge only"
echo "  - Delete branch on merge"
echo ""
echo "Secrets configured:"
gh secret list --repo "$REPO" 2>/dev/null || echo "  (none or no access)"
echo ""
