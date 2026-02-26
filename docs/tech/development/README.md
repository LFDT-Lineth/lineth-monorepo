# Development Guide

> **Diagram:** [Docker Network Topology](../diagrams/docker-network-topology.mmd) (Mermaid source)

## Prerequisites

| Tool | Version | Purpose |
|------|---------|---------|
| Node.js | v22+ | TypeScript projects |
| pnpm | v10.28+ | Package management |
| Docker | v24+ | Container runtime |
| Docker Compose | v2.19+ | Multi-container orchestration |
| Make | v3.81+ | Build automation |
| JDK | 21 | Kotlin/Java projects |
| Gradle | 8.5+ | JVM build system |
| Go | 1.21+ | Prover |

### Resource Requirements

- **Memory**: 16GB+ recommended for full stack
- **CPU**: 4+ cores
- **Disk**: 50GB+ free space

## Quick Start

```bash
# 1. Clone repository
git clone https://github.com/Consensys/linea-monorepo.git
cd linea-monorepo

# 2. Install Node dependencies
make pnpm-install

# 3. Start full stack
make start-env-with-tracing-v2

# 4. Run E2E tests
cd e2e && pnpm run test:e2e:local

# 5. Clean up
make clean-environment
```

## Local Stack Commands

### Starting Services

```bash
# Full L1 + L2 stack with contract deployment
make start-env-with-tracing-v2

# L1 only
make start-l1

# L2 blockchain only (no coordinator/prover)
make start-l2-blockchain-only

# With CI configuration (web3signer enabled)
make start-env-with-tracing-v2-ci

# With fleet (leader + follower nodes)
make start-env-with-tracing-v2-fleet-ci

# With state recovery services
make start-env-with-staterecovery

# With Validium mode
make start-env-with-validium-and-tracing-v2-ci
```

### Stopping Services

```bash
# Clean environment and volumes
make clean-environment

# Full cleanup including Docker volumes
make clean-environment
docker system prune --volumes
```

### Restarting with Previous State

```bash
# Keep existing state
make start-env CLEAN_PREVIOUS_ENV=false
```

## Docker Profiles

### Profile Overview

| Profile | Services | Purpose |
|---------|----------|---------|
| `l1` | l1-el-node, l1-cl-node, genesis | L1 blockchain |
| `l2` | sequencer, maru, coordinator, prover | L2 full stack |
| `l2-bc` | sequencer, maru, nodes | L2 blockchain only |
| `debug` | blockscout, postman, tx-exclusion | Debug tools |
| `staterecovery` | blobscan, recovery nodes | State recovery |
| `observability` | prometheus, grafana, loki | Monitoring |

### Using Profiles

```bash
# Single profile
COMPOSE_PROFILES=l1 docker compose up -d

# Multiple profiles
COMPOSE_PROFILES=l1,l2,debug docker compose up -d

# Via Makefile
make start-env COMPOSE_PROFILES=l1,l2,debug
```

## Building Components

### TypeScript Projects

```bash
# Build all TypeScript packages
pnpm run build

# Build specific package
cd sdk && pnpm run build
cd contracts && pnpm run build
cd postman && pnpm run build

# Lint
pnpm run lint
pnpm run lint:fix

# Test
pnpm run test
```

### Kotlin/Java Projects (Gradle)

```bash
# Build all
./gradlew build

# Build specific project
./gradlew :coordinator:app:build
./gradlew :tracer:arithmetization:build
./gradlew :besu-plugins:linea-sequencer:build

# Test
./gradlew test
./gradlew :coordinator:app:test

# Integration tests
./gradlew integrationTest

# Compile only (faster)
./gradlew compileAll
```

### Go Projects (Prover)

```bash
cd prover

# Build
go build ./...

# Test
go test ./...

# Build specific binaries
go build -o prover ./cmd/prover
go build -o controller ./cmd/controller
```

### Smart Contracts

```bash
cd contracts

# Compile
npx hardhat compile

# Test
npx hardhat test

# Specific test file
npx hardhat test test/hardhat/rollup/LineaRollup.ts

# Coverage
npx hardhat coverage
```

## Docker Images

### Building Local Images

```bash
# Coordinator
./gradlew coordinator:app:installDist
docker buildx build \
  --file coordinator/Dockerfile \
  --build-context libs=./coordinator/app/build/install/coordinator/lib \
  --tag consensys/linea-coordinator:local .

# Use local image
COORDINATOR_TAG=local make start-env-with-tracing-v2
```

### Pulling Pre-built Images

```bash
make docker-pull-images-external-to-monorepo
```

## Contract Deployment

### Local Deployment

```bash
# Deploy all contracts
make deploy-contracts

# Deploy specific contracts
make deploy-linea-rollup-v6
make deploy-l2messageservice
make deploy-token-bridge-l1
make deploy-token-bridge-l2
make deploy-l1-test-erc20
make deploy-l2-test-erc20
```

### Deployment Order

1. PlonkVerifier (L1)
2. LineaRollup (L1)
3. L2MessageService (L2)
4. TokenBridge (L1)
5. TokenBridge (L2)
6. Test tokens (optional)

## Service Endpoints

| Service | URL | Description |
|---------|-----|-------------|
| L1 EL Node | http://localhost:8445 | L1 Execution Layer RPC |
| L1 CL Node | http://localhost:4003 | L1 Consensus Layer REST |
| Sequencer | http://localhost:8545 | L2 Sequencer RPC |
| Maru Engine | http://localhost:8080 | L2 Execution Engine (Engine API) |
| L2 Node Besu | http://localhost:9045 | L2 Besu RPC Node |
| Traces Node | http://localhost:8745 | Trace generation |
| Coordinator | http://localhost:9545 | Coordinator API |
| Postman | http://localhost:9090 | Message relay |
| Shomei | http://localhost:8998 | State manager (Merkle proofs) |
| Transaction Exclusion | http://localhost:8082 | TX exclusion API |
| PostgreSQL | localhost:5432 | Database |
| Blockscout L1 | http://localhost:4001 | L1 Explorer |
| Blockscout L2 | http://localhost:4000 | L2 Explorer |
| Grafana | http://localhost:3001 | Monitoring |
| Prometheus | http://localhost:9091 | Metrics |

> **Note**: Maru and Shomei are external dependencies not in this repository. See [External Dependencies](../architecture/EXTERNAL-DEPENDENCIES.md) for details.

## Monitoring

### Grafana Dashboards

Access: http://localhost:3001

Default dashboards:
- Coordinator Overview
- L2 Transaction Metrics
- Prover Statistics
- Message Bridge Status

### Prometheus Metrics

Access: http://localhost:9091

Key metrics:
- `coordinator_blocks_conflated_total`
- `coordinator_blobs_submitted_total`
- `coordinator_finalizations_submitted_total`
- `sequencer_transactions_processed_total`
- `prover_proof_generation_time_seconds`

### Loki Logs

Access: http://localhost:3100

Query examples:
```
{container="coordinator"}
{container="sequencer"} |= "error"
{container=~"coordinator|sequencer"} | json
```

## Configuration

### Environment Variables

Key environment variables for stack customization:

```bash
# Container image versions
export BESU_PACKAGE_TAG=latest
export MARU_TAG=latest
export COORDINATOR_TAG=latest
export PROVER_TAG=latest
export POSTMAN_TAG=latest

# Coordinator settings
export LINEA_COORDINATOR_SIGNER_TYPE=web3j  # or web3signer
export LINEA_COORDINATOR_DATA_AVAILABILITY=ROLLUP  # or VALIDIUM
export LINEA_COORDINATOR_DISABLE_TYPE2_STATE_PROOF_PROVIDER=true

# Contract addresses (set after deployment)
export L1_ROLLUP_CONTRACT_ADDRESS=0x...
```

### Configuration Files

| File | Location | Purpose |
|------|----------|---------|
| Coordinator config | `config/coordinator/` | Main coordinator settings |
| Prover config | `config/prover/` | Prover settings |
| Traces limits | `config/common/traces-limits-v2.toml` | Conflation limits |
| Gas multipliers | `config/common/gas-price-cap-time-of-day-multipliers.toml` | Gas pricing |
| L1 node | `docker/config/l1-node/` | L1 Besu/Teku config |
| L2 sequencer | `docker/config/linea-besu-sequencer/` | Sequencer config |

### Tuning Conflation

Edit `config/coordinator/coordinator-docker.config.toml`:

```toml
[conflation]
conflation-deadline = "PT6S"  # Time limit (default 6 seconds)
blocks-limit = 200            # Max blocks per batch
```

## Debugging

### Container Logs

```bash
# View logs
docker logs coordinator
docker logs sequencer
docker logs prover-v3

# Follow logs
docker logs -f coordinator

# Multiple services
docker compose -f docker/compose-tracing-v2.yml logs -f coordinator sequencer
```

### Service Health

```bash
# Check service status
docker compose -f docker/compose-tracing-v2.yml ps

# Check specific service health
docker inspect --format='{{.State.Health.Status}}' sequencer
```

### Network Debugging

```bash
# Shell into network
docker run --rm -it --network=docker_linea weibeld/ubuntu-networking bash

# Test connectivity
ping coordinator
curl http://sequencer:8545
```

### Database Access

```bash
# Connect to PostgreSQL
docker exec -it postgres psql -U postgres -d coordinator

# Query batches
SELECT * FROM batches ORDER BY created_at DESC LIMIT 10;
```

## Testing

### Unit Tests

```bash
# TypeScript
pnpm run test

# Kotlin/Java
./gradlew test

# Go
cd prover && go test ./...
```

### Integration Tests

```bash
# Gradle integration tests
./gradlew integrationTest

# Contract tests
cd contracts && npx hardhat test
```

### End-to-End Tests

```bash
# Start stack first
make start-env-with-tracing-v2-ci

# Run E2E tests
cd e2e && pnpm run test:e2e:local

# Specific test
cd e2e && pnpm run test:e2e:local -- messaging.spec.ts

# Fleet tests
cd e2e && pnpm run test:e2e:fleet:local

# Liveness tests
cd e2e && pnpm run test:e2e:liveness:local
```

## Troubleshooting

### Common Issues

**Docker network conflicts:**
```bash
make clean-environment
docker system prune --volumes
```

**Port already in use:**
```bash
lsof -i :8545
kill <PID>
```

**Build failures:**
```bash
# Clear caches
pnpm clean
./gradlew clean
```

**Coordinator connection issues:**
1. Check service health: `docker compose ps`
2. Verify network: `docker network ls`
3. Check logs: `docker logs coordinator`

### Getting Help

- Check [existing documentation](../docs/)
- Review [GitHub issues](https://github.com/Consensys/linea-monorepo/issues)
- Join [Discord](https://discord.gg/linea)

## IDE Setup

### IntelliJ IDEA (Kotlin/Java)

1. Open project root
2. Import Gradle project
3. Set JDK 21
4. Install Kotlin plugin

### VS Code (TypeScript)

1. Open project root
2. Install recommended extensions
3. Run `pnpm install`

### Run Configurations

Example IntelliJ run configurations are in `.run/` directory.
