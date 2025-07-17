# FreqTrade Operator

A Kubernetes operator for managing FreqTrade cryptocurrency trading bots.

## Overview

The FreqTrade Operator provides a Kubernetes-native way to deploy and manage FreqTrade bots. It introduces custom resources for all FreqTrade components and handles the deployment and configuration of the bots.

Key features:
- Deploy FreqTrade bots with custom strategies
- Manage exchange configurations securely using Kubernetes Secrets
- Configure notifications, risk management, and order types
- Deploy a central FreqUI instance for monitoring all bots
- Automatically configure CORS and JWT for secure communication

## Architecture

The operator consists of the following components:

1. **Custom Resource Definitions (CRDs)**:
   - `TradeBot`: Main resource for deploying a FreqTrade bot
   - `FreqUI`: Resource for deploying the FreqUI web interface
   - Supporting resources: `Exchange`, `Strategy`, `PairList`, `EntryPricing`, `ExitPricing`, `OrderTypes`, `RiskManagement`, `Notification`

2. **Controllers**:
   - `TradeBotController`: Manages the lifecycle of TradeBot resources
   - `FreqUIController`: Manages the lifecycle of FreqUI resources

3. **ConfigBuilder**:
   - Assembles FreqTrade configuration from Kubernetes resources
   - Handles secret management for secure credentials

## Prerequisites

- Kubernetes cluster 1.19+
- kubectl 1.19+
- Helm 3+ (optional, for Helm chart installation)
- Go 1.19+ (for building from source)

## Installation

### Using pre-built images

```bash
# Apply CRDs
kubectl apply -f https://github.com/ark-sys/freqtrade-operator/releases/latest/download/crds.yaml

# Deploy the operator
kubectl apply -f https://github.com/ark-sys/freqtrade-operator/releases/latest/download/operator.yaml
```

### Building from source

1. Clone the repository:
```bash
git clone https://github.com/ark-sys/freqtrade-operator.git
cd freqtrade-operator
```

2. Build and deploy the operator:
```bash
# Build the operator image
make docker-build IMG=your-registry/freqtrade-operator:latest

# Push the image to your registry
make docker-push IMG=your-registry/freqtrade-operator:latest

# Deploy the operator to your cluster
make deploy IMG=your-registry/freqtrade-operator:latest
```

## Usage

### 1. Create a namespace for your trading bots

```bash
kubectl create namespace freqtrade
```

### 2. Create secrets for exchange API credentials

```bash
kubectl apply -f examples/exchange-secret.yaml
```

### 3. Create FreqUI deployment

```bash
kubectl apply -f examples/frequi.yaml
```

### 4. Deploy a TradeBot

```bash
# Apply supporting resources
kubectl apply -f examples/exchange.yaml
kubectl apply -f examples/pairlists.yaml
kubectl apply -f examples/strategy.yaml
kubectl apply -f examples/notification.yaml
kubectl apply -f examples/riskmanagement.yaml

# Deploy the bot
kubectl apply -f examples/tradebot.yaml
```

### 5. Access FreqUI

Get the FreqUI URL:
```bash
kubectl get frequi -n trading
```

## Examples

The `examples/` directory contains example resources for deploying a complete FreqTrade setup:

- `namespace.yaml`: Namespace for trading resources
- `exchange-secret.yaml`: Secret for exchange API credentials
- `notification-secret.yaml`: Secret for notification credentials
- `exchange.yaml`: Exchange configuration
- `pairlists.yaml`: Trading pair lists
- `strategy.yaml`: Trading strategy
- `entrypricing.yaml`: Entry pricing configuration
- `exitpricing.yaml`: Exit pricing configuration
- `ordertypes.yaml`: Order types configuration
- `riskmanagement.yaml`: Risk management configuration
- `notification.yaml`: Notification configuration
- `tradebot.yaml`: TradeBot deployment
- `frequi.yaml`: FreqUI deployment

## Development

### Prerequisites

- Go 1.19+
- Docker
- Kubernetes cluster (or minikube/kind)
- Operator SDK

### Setup Development Environment

1. Install the required tools:
```bash
# Install controller-gen
make controller-gen

# Install kustomize
make kustomize
```

2. Generate manifests and code:
```bash
make manifests
make generate
```

3. Run the operator locally:
```bash
make run
```

### Running Tests

```bash
# Run unit tests
make test

# Run e2e tests
make test-e2e
```

## License

This project is licensed under the Apache 2.0 License - see the LICENSE file for details.
