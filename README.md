# PAVE Fees API

Billing API built with Encore and Temporal for managing fees.

## Setup

### Prerequisites

- Go 1.21+
- Encore CLI
- Temporal CLI

### [Install Encore CLI](https://encore.dev/docs/ts/install#install-the-encore-cli)

```bash
brew install encoredev/tap/encore
```

### [Install Temporal CLI](https://docs.temporal.io/cli#install)

```bash
brew install temporal
```

### Run

1. Start Temporal server

```bash
temporal server start-dev --namespace default --db-filename temporal.db
```

2. Start Encore

```bash
encore run
```

3. Initialize Temporal search attributes (first run only)

```bash
temporal operator search-attribute create --name CustomerID --type Int
```
