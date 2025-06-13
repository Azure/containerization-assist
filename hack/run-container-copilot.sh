#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<EOF
Usage: $0 [flags]

Flags (override .env / env):
  -g <RESOURCE_GROUP>        Azure resource group
  -l <LOCATION>              Azure region for the resource group
  -a <OPENAI_RES_NAME>       Azure OpenAI Cognitive Services resource name
  -d <DEPLOYMENT_NAME>       Deployment name
  -m <MODEL_ID>              Model ID (default: o3-mini)
  -v <MODEL_VERSION>         Model version (default: 2025-01-31)
  -t <TARGET_REPO>           Path to the repo to containerize
  --setup-only               Set up resources only, don't run the generator
  -h                         Show this help message
EOF
  exit 1
}

# 1) locate dirs
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# 2) source .env from project root, then hack/
if [ -f "$PROJECT_ROOT/.env" ]; then
  echo "→ Sourcing $PROJECT_ROOT/.env"
  set -o allexport; source "$PROJECT_ROOT/.env"; set +o allexport
fi
if [ -f "$SCRIPT_DIR/.env" ]; then
  echo "→ Sourcing $SCRIPT_DIR/.env"
  set -o allexport; source "$SCRIPT_DIR/.env"; set +o allexport
fi

# 3) parse flags
FLAG_RG="" FLAG_LOC="" FLAG_RES="" FLAG_DEP="" FLAG_MOD="" FLAG_VER="" FLAG_TR=""
SETUP_ONLY=false
DEFAULT_MODEL="o3-mini"
DEFAULT_MODEL_VERSION="2025-01-31"

# Handle the --setup-only flag separately
for arg in "$@"; do
  if [[ "$arg" == "--setup-only" ]]; then
    SETUP_ONLY=true
  fi
done

while getopts "g:l:a:d:m:v:t:h-:" opt; do
  case "$opt" in
    g) FLAG_RG="$OPTARG" ;;
    l) FLAG_LOC="$OPTARG" ;;
    a) FLAG_RES="$OPTARG" ;;
    d) FLAG_DEP="$OPTARG" ;;
    m) FLAG_MOD="$OPTARG" ;;
    v) FLAG_VER="$OPTARG" ;;
    t) FLAG_TR="$OPTARG" ;;
    h) usage ;;
    -) case "${OPTARG}" in
         setup-only) SETUP_ONLY=true ;;
         *) usage ;;
       esac ;;
    *) usage ;;
  esac
done
shift $((OPTIND -1))

# 4) final assignment: flags → .env/env → defaults
RG="${FLAG_RG:-${AZURE_OPENAI_RESOURCE_GROUP:-}}"
LOCATION="${FLAG_LOC:-${AZURE_OPENAI_LOCATION:-}}"
RES="${FLAG_RES:-${AZURE_OPENAI_RESOURCE_NAME:-}}"
DEPLOY="${FLAG_DEP:-${AZURE_OPENAI_DEPLOYMENT_NAME:-}}"
MODEL_ID="${FLAG_MOD:-${AZURE_OPENAI_MODEL_ID:-$DEFAULT_MODEL}}"
MODEL_VERSION="${FLAG_VER:-${AZURE_OPENAI_MODEL_VERSION:-$DEFAULT_MODEL_VERSION}}"
TARGET_REPO="${FLAG_TR:-${TARGET_REPO:-}}"

# 5) validate required AZURE_OPENAI_* settings
MISSING=()
for VAR in RG LOCATION RES DEPLOY MODEL_ID MODEL_VERSION TARGET_REPO; do
  if [ -z "${!VAR}" ]; then
    MISSING+=("$VAR")
  fi
done
if [ "${#MISSING[@]}" -gt 0 ]; then
  echo "✗ Missing required values: ${MISSING[*]}"
  usage
fi

echo "→ Configuration:
  RESOURCE_GROUP:        $RG
  LOCATION:              $LOCATION
  OPENAI_RES_NAME:       $RES
  DEPLOYMENT_NAME:       $DEPLOY
  MODEL_ID:              $MODEL_ID
  MODEL_VERSION:         $MODEL_VERSION
  TARGET_REPO:           $TARGET_REPO
  SETUP_ONLY:            $SETUP_ONLY
"

# 6) check prerequisites
echo "→ Verifying prerequisites…"
for CMD in az go kubectl docker kind; do
  command -v "$CMD" &>/dev/null || { echo "✗ $CMD not found"; exit 1; }
  echo "✓ $CMD"
done

# 7) ensure RG
echo -e "\n→ Checking resource group '$RG'…"
if ! az group show --name "$RG" &>/dev/null; then
  echo "  not found → creating in '$LOCATION'…"
  az group create --name "$RG" --location "$LOCATION" --output none
  echo "  ✓ Created"
else
  echo "  ✓ Exists"
fi

# 8) ensure OpenAI Cognitive Services account
echo -e "\n→ Ensuring Cognitive Services account '$RES' (kind=OpenAI)…"
if ! az cognitiveservices account show \
      --name "$RES" \
      --resource-group "$RG" &>/dev/null; then
  echo "  not found → creating…"
  az cognitiveservices account create \
    --name "$RES" \
    --resource-group "$RG" \
    --kind OpenAI \
    --sku S0 \
    --location "$LOCATION" \
    --yes \
    --output none
  echo "  ✓ Created account"
else
  echo "  ✓ Account exists"
fi

# 9) fetch API key & endpoint if not set
echo -e "\n→ Retrieving API key and endpoint…"
if [ -z "${AZURE_OPENAI_KEY:-}" ]; then
  export AZURE_OPENAI_KEY=$(az cognitiveservices account keys list \
    --name "$RES" \
    --resource-group "$RG" \
    --query "key1" -o tsv) && echo "  ✓ Key retrieved"
fi
if [ -z "${AZURE_OPENAI_ENDPOINT:-}" ]; then
  export AZURE_OPENAI_ENDPOINT=$(az cognitiveservices account show \
    --name "$RES" \
    --resource-group "$RG" \
    --query "properties.endpoint" -o tsv) && echo "  ✓ Endpoint retrieved"
fi

# 10) list available models
echo -e "\n→ Available models on '$RES':"
az cognitiveservices account list-models \
  --resource-group "$RG" \
  --name "$RES" \
  --output table

# 11) create/update deployment (use GlobalStandard SKU)
echo -e "\n→ Creating/updating deployment '$DEPLOY'…"
az cognitiveservices account deployment create \
  --name            "$RES" \
  --resource-group  "$RG" \
  --deployment-name "$DEPLOY" \
  --model-name      "$MODEL_ID" \
  --model-version   "$MODEL_VERSION" \
  --model-format    OpenAI \
  --sku-name        GlobalStandard \
  --sku-capacity    10 \
  --only-show-errors \
  --output none

echo "  ✓ Deployment '$DEPLOY' ready"

# 12) export AZURE_* for container‑copilot
echo -e "\n→ Exporting AZURE_* variables…"
export AZURE_OPENAI_KEY
export AZURE_OPENAI_ENDPOINT
export AZURE_OPENAI_DEPLOYMENT_ID="$DEPLOY"

# If setup-only is true, don't run the generator
if [ "$SETUP_ONLY" = true ]; then
  echo -e "\n✓ Setup complete. Azure OpenAI resources are ready."
  exit 0
fi

# 13) run the generator
echo -e "\n→ Running container‑copilot on '$TARGET_REPO'…"
(
  cd "$PROJECT_ROOT"
  go run . generate "$TARGET_REPO"
)

echo -e "\n🎉 All done! Artifacts in '$TARGET_REPO'."
