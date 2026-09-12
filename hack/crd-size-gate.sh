#!/usr/bin/env bash
# G5-1/§2.1 (GATEWAY-API-PLAN.md): frequis.freqtrade.io carries two served versions, both
# inlining large pod-shaped types, and already sits close to kubectl's 262 KB
# last-applied-configuration annotation limit (which is why this CRD has always had to be
# installed via Helm's crds/ directory or `kubectl apply --server-side`, not plain
# `kubectl apply` - that is the pre-existing status quo, not something this gate changes).
# D4/D5 chose to embed gatewayv1.ParentReference/Hostname (~9 KB/served version) rather than a
# raw []gatewayv1.HTTPRouteRule passthrough (~195 KB/served version) specifically to keep this
# CRD nowhere near etcd's ~1.5 MB value limit. Run after `make manifests`. Fail loudly - not
# silently truncate or best-effort ignore - if a future field addition (most plausibly:
# reintroducing an HTTPRouteRule-shaped escape hatch D5 rejected) blows that budget again.
set -euo pipefail

cd "$(dirname "$0")/.."

CRD_FILE="config/crd/bases/freqtrade.io_frequis.yaml"
MAX_BYTES=1000000

if [[ ! -f "$CRD_FILE" ]]; then
  echo "error: ${CRD_FILE} not found - run 'make manifests' first" >&2
  exit 1
fi

actual_bytes=$(wc -c <"$CRD_FILE")
echo "${CRD_FILE}: ${actual_bytes} bytes (ceiling: ${MAX_BYTES} bytes)"

if ((actual_bytes > MAX_BYTES)); then
  echo "FAIL: ${CRD_FILE} is ${actual_bytes} bytes, over the ${MAX_BYTES}-byte ceiling (§2.1," >&2
  echo "GATEWAY-API-PLAN.md). This almost always means a new field was inlined at full schema" >&2
  echo "size instead of referencing an existing type, or D5's rejected HTTPRouteRule" >&2
  echo "passthrough crept back in - see §2.1's measured cost table before adding it back." >&2
  exit 1
fi
