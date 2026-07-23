#!/usr/bin/env bash
# Maven reachability probe. Runs INSIDE a `docker build` (same network context
# as a real Maven build in the agent eval). Curls every candidate Maven endpoint
# with every candidate credential and prints the HTTP status, so we can pick the
# endpoint + auth that actually works before wiring the real settings.xml mirror.
#
# The Azure DevOps OAuth token is read from the BuildKit secret at
# /run/secrets/aztoken (never baked into an image layer).
set -u

TOKEN="$(cat /run/secrets/aztoken 2>/dev/null || echo '')"
# Representative Maven Central coordinate (Spring Boot parent POM used by fixtures).
ARTIFACT="org/springframework/boot/spring-boot-starter-parent/2.7.18/spring-boot-starter-parent-2.7.18.pom"

hit() {
  # hit <label> <url> [extra curl args...]
  label="$1"
  url="$2"
  shift 2
  code="$(curl -s -o /dev/null -w '%{http_code}' --max-time 12 "$@" "$url" 2>/dev/null || echo 'ERR')"
  printf '  %-48s -> %s\n' "$label" "$code"
}

echo '=== MAVEN PROBE ==='
echo "token present: $([ -n "$TOKEN" ] && echo "yes (${#TOKEN} chars)" || echo no)"

echo '--- controls (sanity) ---'
hit 'mcr.microsoft.com/v2/ (expect 2xx/3xx)' 'https://mcr.microsoft.com/v2/'
hit 'repo.maven.apache.org (expect blocked/ERR)' "https://repo.maven.apache.org/maven2/${ARTIFACT}"

echo '--- CFS proxy: packagefeedproxy.microsoft.io ---'
for base in \
  'https://packagefeedproxy.microsoft.io/maven/v1' \
  'https://packagefeedproxy.microsoft.io/maven2' \
  'https://packagefeedproxy.microsoft.io/maven'; do
  hit "${base}  [noauth]" "${base}/${ARTIFACT}"
  hit "${base}  [bearer]" "${base}/${ARTIFACT}" -H "Authorization: Bearer ${TOKEN}"
  hit "${base}  [basic]"  "${base}/${ARTIFACT}" -u ".:${TOKEN}"
done

echo '--- Azure Artifacts host reachability (feed name TBD) ---'
hit 'pkgs.dev.azure.com root [noauth]' 'https://pkgs.dev.azure.com/AzureContainerUpstream/'
hit 'pkgs.dev.azure.com root [bearer]' 'https://pkgs.dev.azure.com/AzureContainerUpstream/' -H "Authorization: Bearer ${TOKEN}"

echo '--- Azure Artifacts Maven feed: Kubernetes_PublicPackages (has Maven Central upstream) ---'
# Real target: mirror `central` through this feed. Proven reachable + serving
# Central artifacts locally; this confirms the BUILD IDENTITY (System.AccessToken)
# can read it from inside `docker build`.
FEED='https://pkgs.dev.azure.com/AzureContainerUpstream/Kubernetes/_packaging/Kubernetes_PublicPackages/maven/v1'
for probe_artifact in \
  "org/springframework/spring-core/maven-metadata.xml" \
  "${ARTIFACT}"; do
  hit "feed noauth: ${probe_artifact}" "${FEED}/${probe_artifact}"
  hit "feed bearer: ${probe_artifact}" "${FEED}/${probe_artifact}" -H "Authorization: Bearer ${TOKEN}"
  hit "feed basic : ${probe_artifact}" "${FEED}/${probe_artifact}" -u ".:${TOKEN}"
done

echo '=== END MAVEN PROBE ==='
