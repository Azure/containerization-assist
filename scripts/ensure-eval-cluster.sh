#!/usr/bin/env bash
#
# Idempotent preflight for the agent-eval AKS cluster: reuse a healthy cluster
# (`<base>`, `<base>-2`, …) or create the next-indexed one instead of waiting on
# a Deleting cluster, then wire ACR pull, refresh kubeconfig and ensure the
# namespace. Prints `RESOLVED_CLUSTER=<name>` on stdout. Inputs are
# env-overridable; defaults match the ca-eval-* setup.
set -euo pipefail

RG="${AGENT_EVAL_RESOURCE_GROUP:-ca-test-suite}"
BASE="${AGENT_EVAL_CLUSTER:-ca-eval-aks}"
# Cluster lives in eastus2 (team sub allows no general VM SKUs in eastus, where
# the RG/ACR are). Standard_D4as_v7 is amd64 to match the harness images.
LOCATION="${AGENT_EVAL_LOCATION:-eastus2}"
NODE_VM_SIZE="${AGENT_EVAL_NODE_VM_SIZE:-Standard_D4as_v7}"
NODE_COUNT="${AGENT_EVAL_NODE_COUNT:-3}"
# Empty = let AKS pick its default (don't pin a version that may be unavailable).
K8S_VERSION="${AGENT_EVAL_K8S_VERSION:-}"
NAMESPACE="${AGENT_EVAL_NAMESPACE:-eval-ns}"
REGISTRY="${AGENT_EVAL_REGISTRY:-caevalacr.azurecr.io}"
ACR_NAME="${REGISTRY%%.*}"

log() { printf '[ensure-cluster] %s\n' "$*" >&2; }

# BASE -> 1, BASE-N -> N, otherwise 0 (not one of ours).
idx_of() {
  local n="$1"
  if [[ "$n" == "$BASE" ]]; then echo 1; return; fi
  local base_re="${BASE//./\\.}"
  if [[ "$n" =~ ^${base_re}-([0-9]+)$ ]]; then echo "${BASH_REMATCH[1]}"; return; fi
  echo 0
}
name_for_idx() { local i="$1"; if [[ "$i" -le 1 ]]; then echo "$BASE"; else echo "${BASE}-${i}"; fi; }

if ! az group show -n "$RG" >/dev/null 2>&1; then
  log "resource group $RG missing — creating in $LOCATION"
  az group create -n "$RG" -l "$LOCATION" >/dev/null
fi

mapfile -t rows < <(az aks list -g "$RG" --query "[].{n:name,s:provisioningState}" -o tsv 2>/dev/null || true)

best_usable=""; best_usable_idx=-1; max_idx=0
for row in "${rows[@]}"; do
  [[ -z "$row" ]] && continue
  name="${row%%$'\t'*}"; state="${row##*$'\t'}"
  i="$(idx_of "$name")"
  [[ "$i" -eq 0 ]] && continue
  (( i > max_idx )) && max_idx="$i"
  # Succeeded/Updating are usable; Deleting/Failed/Creating are not.
  if [[ "$state" == "Succeeded" || "$state" == "Updating" ]]; then
    if (( i > best_usable_idx )); then best_usable="$name"; best_usable_idx="$i"; fi
  else
    log "skipping $name (state=$state)"
  fi
done

if [[ -n "$best_usable" ]]; then
  CLUSTER="$best_usable"
  log "reusing healthy cluster $CLUSTER"
else
  if (( max_idx == 0 )); then next_idx=1; else next_idx=$(( max_idx + 1 )); fi
  CLUSTER="$(name_for_idx "$next_idx")"
  log "no healthy cluster — creating fresh $CLUSTER (loc=$LOCATION vm=$NODE_VM_SIZE count=$NODE_COUNT k8s=${K8S_VERSION:-default})"
  k8s_args=()
  [[ -n "$K8S_VERSION" ]] && k8s_args=(--kubernetes-version "$K8S_VERSION")
  # No --attach-acr (needs Owner to create the AcrPull role assignment); ACR pull
  # is wired via an imagePullSecret below instead.
  az aks create -g "$RG" -n "$CLUSTER" \
    --location "$LOCATION" \
    --node-count "$NODE_COUNT" \
    --node-vm-size "$NODE_VM_SIZE" \
    "${k8s_args[@]}" \
    --generate-ssh-keys >/dev/null
  log "created $CLUSTER"
fi

az aks get-credentials -g "$RG" -n "$CLUSTER" --overwrite-existing >/dev/null

kubectl get namespace "$NAMESPACE" >/dev/null 2>&1 || kubectl create namespace "$NAMESPACE" >/dev/null

log "assuming AcrPull is granted to the AKS kubelet identity on $ACR_NAME — pods will ImagePullBackOff if that role is missing"

log "ready: cluster=$CLUSTER ns=$NAMESPACE"
printf 'RESOLVED_CLUSTER=%s\n' "$CLUSTER"
