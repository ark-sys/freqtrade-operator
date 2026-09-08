#!/usr/bin/env bash
# Syncs generated CRDs and RBAC from config/ into the Helm chart, so
# helm/crds/*.yaml and helm/templates/clusterrole.yaml stop drifting from
# what `make manifests` actually generates. Run via `make helm-sync`, after
# `make manifests` has already regenerated config/crd/bases and
# config/rbac/role.yaml.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

cp config/crd/bases/*.yaml helm/crds/

{
	echo '{{- if .Values.rbac.create -}}'
	echo 'apiVersion: rbac.authorization.k8s.io/v1'
	echo 'kind: ClusterRole'
	echo 'metadata:'
	echo '  name: {{ include "freqtrade-operator.fullname" . }}-manager-role'
	echo '  labels:'
	echo '    {{- include "freqtrade-operator.labels" . | nindent 4 }}'
	sed -n '/^rules:/,$p' config/rbac/role.yaml
	echo '{{- end }}'
} >helm/templates/clusterrole.yaml
