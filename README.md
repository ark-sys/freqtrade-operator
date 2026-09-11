# FreqTrade Operator

A Kubernetes operator for managing FreqTrade cryptocurrency trading bots.

> **Disclaimer.** This software deploys and manages automated cryptocurrency trading systems. Running any of these
> resources with `dry_run: false` and real exchange credentials places real funds at risk of loss through strategy
> behavior, exchange conditions, software defects, or misconfiguration. The maintainers and contributors provide this
> software "as is", without warranty of any kind (see [LICENSE](LICENSE)), and are not responsible for financial
> losses incurred through its use. You are solely responsible for the strategies you run, the credentials you grant
> this operator, and the funds under their control. See [SECURITY.md](SECURITY.md) for the threat model this design
> assumes.

## Overview

The FreqTrade Operator provides a Kubernetes-native way to deploy and manage FreqTrade bots. It introduces custom resources for all FreqTrade components and handles the deployment and configuration of the bots.

Key features:
- Deploy FreqTrade bots with custom strategies
- Manage exchange configurations securely using Kubernetes Secrets
- Configure notifications, risk management, and order types
- Deploy a central FreqUI instance for monitoring all bots
- Automatically configure CORS and JWT for secure communication

## Install

The Helm chart, published as an OCI artifact on every release, is the preferred way to install:

```bash
helm install freqtrade-operator oci://ghcr.io/ark-sys/freqtrade-operator --version <chart-version>
```

See [docs/installation.md](docs/installation.md) for prerequisites, plain-manifest (`kubectl apply`) and OLM
installs, building from source, and verifying signed release images.

## Documentation

| | |
|---|---|
| [docs/installation.md](docs/installation.md) | Prerequisites, install methods, verifying signed images |
| [docs/usage.md](docs/usage.md) | Deploying a TradeBot, exchange/API credentials, config changes & restarts, starting/stopping a bot |
| [docs/backtesting.md](docs/backtesting.md) | One-shot backtest runs, parameter sweeps, reading results |
| [docs/monitoring.md](docs/monitoring.md) | Prometheus metrics, bot introspection |
| [docs/operations.md](docs/operations.md) | Config Secret backups, NetworkPolicy, production checklist |
| [docs/architecture.md](docs/architecture.md) | CRDs, controllers, namespace model, RBAC, design decisions |
| [docs/api-reference.md](docs/api-reference.md) | Every field on every CRD, generated from source |
| [docs/upgrading.md](docs/upgrading.md) | Migrating an existing install to `v1beta1` |
| [docs/troubleshooting.md](docs/troubleshooting.md) | Concrete symptoms and what to do about them |
| [examples/](examples/README.md) | Complete, self-contained kustomizations to apply as-is |

## Development

See [CONTRIBUTING.md](CONTRIBUTING.md) for building, running, and testing the operator locally.

## Changelog

See [CHANGELOG.md](CHANGELOG.md) for notable changes to this project.

## License

This project is licensed under the Apache 2.0 License - see the LICENSE file for details.
