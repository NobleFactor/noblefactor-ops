#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
# Copyright (c) 2025 Noble Factor. All rights reserved.
#
# Azure Static Web App Setup Script
# Creates resource group, SWA, and Entra ID app for authentication
#
# Usage:
#   ./setup-azure-swa.sh --name <app-name> --domain <custom-domain>
#
# Example:
#   ./setup-azure-swa.sh --name devlore-site --domain devlore.noblefactor.com
#
# Prerequisites:
#   - Azure CLI installed and logged in (az login)
#   - Subscription with permissions to create resources
#
# Environment variables (optional):
#   AZURE_SUBSCRIPTION    - Subscription ID (default: current subscription)
#   AZURE_LOCATION        - Azure region (default: westus2)
#   AZURE_RESOURCE_GROUP  - Resource group name (default: rg-<app-name>)

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
error() { echo -e "${RED}error:${NC} $*" >&2; exit 1; }

# Parse arguments
APP_NAME=""
CUSTOM_DOMAIN=""
while [[ $# -gt 0 ]]; do
    case $1 in
        --name) APP_NAME="$2"; shift 2 ;;
        --domain) CUSTOM_DOMAIN="$2"; shift 2 ;;
        --help|-h)
            echo "Usage: $0 --name <app-name> --domain <custom-domain>"
            echo ""
            echo "Options:"
            echo "  --name     Application name (used for resource naming)"
            echo "  --domain   Custom domain (e.g., devlore.noblefactor.com)"
            echo ""
            echo "Environment variables:"
            echo "  AZURE_SUBSCRIPTION    Subscription ID"
            echo "  AZURE_LOCATION        Azure region (default: westus2)"
            echo "  AZURE_RESOURCE_GROUP  Resource group name"
            exit 0
            ;;
        *) error "Unknown option: $1" ;;
    esac
done

[[ -z "$APP_NAME" ]] && error "Missing required --name argument"
[[ -z "$CUSTOM_DOMAIN" ]] && error "Missing required --domain argument"

# Configuration
LOCATION="${AZURE_LOCATION:-westus2}"
RESOURCE_GROUP="${AZURE_RESOURCE_GROUP:-rg-${APP_NAME}}"
SWA_NAME="${APP_NAME}"
ENTRA_APP_NAME="${APP_NAME}-auth"

# Check prerequisites
info "Checking prerequisites..."

if ! command -v az &>/dev/null; then
    error "Azure CLI not found. Install from https://docs.microsoft.com/en-us/cli/azure/install-azure-cli"
fi

if ! az account show &>/dev/null; then
    error "Not logged in to Azure. Run 'az login' first."
fi

# Get subscription info
SUBSCRIPTION="${AZURE_SUBSCRIPTION:-$(az account show --query id -o tsv)}"
SUBSCRIPTION_NAME=$(az account show --query name -o tsv)
TENANT_ID=$(az account show --query tenantId -o tsv)

info "Subscription: $SUBSCRIPTION_NAME"
info "Tenant ID: $TENANT_ID"
info "Location: $LOCATION"
info "Resource Group: $RESOURCE_GROUP"
info "SWA Name: $SWA_NAME"
echo ""

# Confirm
read -p "Continue with these settings? [y/N] " -n 1 -r
echo
[[ ! $REPLY =~ ^[Yy]$ ]] && exit 1

# Phase 1: Create Resource Group
info "Creating resource group..."
if az group show --name "$RESOURCE_GROUP" &>/dev/null; then
    warn "Resource group $RESOURCE_GROUP already exists"
else
    az group create --name "$RESOURCE_GROUP" --location "$LOCATION" --output none
    success "Created resource group: $RESOURCE_GROUP"
fi

# Phase 2: Create Static Web App
info "Creating Static Web App..."
if az staticwebapp show --name "$SWA_NAME" --resource-group "$RESOURCE_GROUP" &>/dev/null; then
    warn "Static Web App $SWA_NAME already exists"
else
    # Standard SKU required for custom authentication
    az staticwebapp create \
        --name "$SWA_NAME" \
        --resource-group "$RESOURCE_GROUP" \
        --location "$LOCATION" \
        --sku Standard \
        --output none
    success "Created Static Web App: $SWA_NAME"
fi

# Get SWA default hostname
SWA_HOSTNAME=$(az staticwebapp show --name "$SWA_NAME" --resource-group "$RESOURCE_GROUP" --query defaultHostname -o tsv)
info "SWA hostname: $SWA_HOSTNAME"

# Phase 3: Create Entra ID App Registration
info "Creating Entra ID app registration..."
EXISTING_APP=$(az ad app list --display-name "$ENTRA_APP_NAME" --query "[0].appId" -o tsv 2>/dev/null || true)

if [[ -n "$EXISTING_APP" ]]; then
    warn "Entra ID app $ENTRA_APP_NAME already exists (appId: $EXISTING_APP)"
    APP_ID="$EXISTING_APP"
else
    # Create app with redirect URIs for SWA authentication
    APP_ID=$(az ad app create \
        --display-name "$ENTRA_APP_NAME" \
        --web-redirect-uris \
            "https://${SWA_HOSTNAME}/.auth/login/aad/callback" \
            "https://${CUSTOM_DOMAIN}/.auth/login/aad/callback" \
        --sign-in-audience AzureADMyOrg \
        --query appId -o tsv)
    success "Created Entra ID app: $APP_ID"
fi

# Create client secret
info "Creating client secret..."
CLIENT_SECRET=$(az ad app credential reset \
    --id "$APP_ID" \
    --display-name "SWA Auth Secret" \
    --years 2 \
    --query password -o tsv)
success "Created client secret"

# Phase 4: Get deployment token
info "Getting deployment token..."
DEPLOYMENT_TOKEN=$(az staticwebapp secrets list \
    --name "$SWA_NAME" \
    --resource-group "$RESOURCE_GROUP" \
    --query "properties.apiKey" -o tsv)

# Phase 5: Create service principal for GitHub Actions
info "Creating service principal for GitHub Actions..."
SP_NAME="sp-${APP_NAME}-github"
EXISTING_SP=$(az ad sp list --display-name "$SP_NAME" --query "[0].appId" -o tsv 2>/dev/null || true)

if [[ -n "$EXISTING_SP" ]]; then
    warn "Service principal $SP_NAME already exists"
    SP_APP_ID="$EXISTING_SP"
    # Reset credentials
    SP_CREDENTIALS=$(az ad sp credential reset --id "$SP_APP_ID" --query "{clientId:appId, clientSecret:password, tenantId:'$TENANT_ID', subscriptionId:'$SUBSCRIPTION'}" -o json)
else
    # Create SP with Contributor role scoped to resource group
    SP_CREDENTIALS=$(az ad sp create-for-rbac \
        --name "$SP_NAME" \
        --role Contributor \
        --scopes "/subscriptions/${SUBSCRIPTION}/resourceGroups/${RESOURCE_GROUP}" \
        --query "{clientId:appId, clientSecret:password, tenantId:tenant, subscriptionId:'$SUBSCRIPTION'}" -o json)
    success "Created service principal: $SP_NAME"
fi

# Output summary
echo ""
echo "=============================================="
echo "  Azure Static Web App Setup Complete"
echo "=============================================="
echo ""
echo "Resource Group:    $RESOURCE_GROUP"
echo "Static Web App:    $SWA_NAME"
echo "SWA Hostname:      $SWA_HOSTNAME"
echo "Custom Domain:     $CUSTOM_DOMAIN"
echo "Entra App ID:      $APP_ID"
echo ""
echo "=============================================="
echo "  GitHub Secrets (store these securely)"
echo "=============================================="
echo ""
echo "AZURE_STATIC_WEB_APPS_API_TOKEN:"
echo "$DEPLOYMENT_TOKEN"
echo ""
echo "AZURE_CREDENTIALS:"
echo "$SP_CREDENTIALS"
echo ""
echo "AAD_CLIENT_ID:"
echo "$APP_ID"
echo ""
echo "AAD_CLIENT_SECRET:"
echo "$CLIENT_SECRET"
echo ""
echo "=============================================="
echo "  DNS Configuration"
echo "=============================================="
echo ""
echo "Add these DNS records at your registrar:"
echo ""
echo "Type:  CNAME"
echo "Name:  ${CUSTOM_DOMAIN%%.*}"
echo "Value: $SWA_HOSTNAME"
echo ""
echo "After DNS propagates, register the domain:"
echo "  az staticwebapp hostname set --name $SWA_NAME --resource-group $RESOURCE_GROUP --hostname $CUSTOM_DOMAIN"
echo ""
echo "=============================================="
echo "  Next Steps"
echo "=============================================="
echo ""
echo "1. Add GitHub secrets using setup-github-repo.sh"
echo "2. Configure DNS records"
echo "3. Register custom domain with Azure"
echo "4. Deploy your site"
echo ""
