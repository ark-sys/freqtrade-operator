#!/usr/bin/env bash
# G0-2 (GATEWAY-API-PLAN.md): sigs.k8s.io/gateway-api is pinned to v1.3.0 deliberately - it is the
# newest release whose own requirements (k8s.io/* v0.32.3, controller-runtime v0.20.4) sit under
# this repo's existing pins (k8s.io/* v0.33.0, controller-runtime v0.21.0), so adding it changes
# nothing else via minimum version selection. v1.4.1+ each force a k8s.io/* and/or
# controller-runtime bump - see the table in GATEWAY-API-PLAN.md §1.4. Dependabot/Renovate (or a
# plain `go get -u`) bumping this past v1.3.0 would silently drag that bump in with it, so this
# gate fails loudly instead.
set -euo pipefail

cd "$(dirname "$0")/.."

WANT="v1.3.0"
GOT="$(go list -m -f '{{.Version}}' sigs.k8s.io/gateway-api)"

if [[ "${GOT}" != "${WANT}" ]]; then
  echo "sigs.k8s.io/gateway-api is pinned to ${WANT} on purpose (GATEWAY-API-PLAN.md §1.4)," >&2
  echo "but go.mod now resolves it to ${GOT}. A version bump here can silently drag the" >&2
  echo "k8s.io/* and/or controller-runtime stack forward via minimum version selection -" >&2
  echo "that is a separate, deliberate piece of work, not something to pick up incidentally." >&2
  exit 1
fi
