#!/bin/bash

IMG="localhost:5001/freqtrade-operator:latest"
cd "$(dirname "$0")/.."

create_cluster() {
  ./hack/kind-test-cluster.sh || { echo "Failed to create kind cluster"; exit 1; }
}

build_image() {
  make generate && make manifests
  IMG="${IMG}" make docker-build || { echo "Failed to build Docker image"; exit 1; }
}

push_image() {
  docker push "${IMG}" || { echo "Failed to push Docker image"; exit 1; }
}

deploy_operator() {
  IMG="${IMG}" make deploy || { echo "Failed to deploy Freqtrade Operator"; exit 1; }
}

wait_for_operator() {
  kubectl wait --for=condition=Available --timeout=120s deployment -l app.kubernetes.io/name=freqtrade-operator -n freqtrade-operator-system || { echo "Operator did not become available in time"; exit 1; }
}

deploy_examples() {
  kubectl apply -k examples/
}

main() {
  kind delete cluster
  create_cluster &
  build_image &
  wait
  push_image
  deploy_operator
  wait_for_operator
  deploy_examples
}

main