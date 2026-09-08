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
   - `TradeBotConfig`: Configuration file for a TradeBot
   - `Strategy`: Python script of the Strategy run by the bot
   - `FreqUI`: Web interface for monitoring and managing the bot

2. **Controllers**:
   - `TradeBotController`: Manages the lifecycle of TradeBot resources
   - `TradeBotConfigController`: Manages the lifecycle of TradeBotConfig resources.
   - `StrategyController`: Manages the lifecycle of Strategy resources.
   - `FreqUIController`: Manages the lifecycle of FreqUI resources and updates CORS settings on referenced TradeBots.

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

### Using Helm

```bash
helm repo add ark-sys https://ark-sys.github.io/freqtrade-operator/
helm repo update
helm install freqtrade-operator ark-sys/freqtrade-operator
```

## Usage

### 1. Create a namespace for your trading bots

```bash
kubectl create namespace freqtrade
```

### 2. Create secrets for exchange API credentials

```bash
kubectl apply -f examples/binance-credentials.yaml
```

### 3. Create FreqUI deployment

```bash
kubectl apply -f examples/frequi.yaml
```

### 4. Deploy a TradeBot

```bash
# Apply supporting resources
kubectl apply -f examples/strategy.yaml
kubectl apply -f examples/config.yaml

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
- `binance-credentials.yaml`: Secret for exchange API credentials
- `config.yaml`: Configuration file for a TradeBot
- `strategy.yaml`: Python script of the Strategy run by the bot
- `tradebot.yaml`: TradeBot deployment
- `frequi.yaml`: FreqUI deployment


## Different ways to deploy a TradeBot 

The default command executed by an instance of a TradeBot is `trade`. This command materializes the TradeBot resource as a StatefulSet that binds configuration and strategy from referenced resources.
The `trade` command can be overridden by setting the `command` field in the TradeBot resource. This allows for different ways to deploy a TradeBot.

### Using the `trade` command

The `trade` command is used to run a live trading bot. It can be used to trade a strategy in a live environment.
This command materializes the TradeBot resource as a StatefulSet that binds configuration and strategy from referenced resources.
The `trade` command is used when no command is specified.

### Using the `backtesting` command

The `backtesting` command is used to backtest a strategy. It can be used to test a strategy before deploying it to a live environment.
This command materializes the TradeBot resource as a Job that runs the backtesting command. 

### Using the `hyperopt` command

The `hyperopt` command is used to run a hyperopt. It can be used to find the best parameters for a strategy.
This command materializes the TradeBot resource as a Job that runs the hyperopt command.

### Using the `plot` command

The `plot` command is used to plot the results of a backtest. It can be used to visualize the results of a backtest.
This command materializes the TradeBot resource as a Job that runs the plot command.

### Deploying an AI bot

To enable this mode, the `trade` command must be provided with the `--freqaimodel` flag.

This command materializes the TradeBot resource as a StatefulSet that binds configuration and strategy from referenced resources.
Also, this resource will look for annotation to determine if a GPU is to be used. If a GPU is available, the TradeBot container will be setup with GPU support.






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
