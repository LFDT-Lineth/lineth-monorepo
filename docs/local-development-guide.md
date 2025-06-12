# Local Development Guide

This guide provides instructions for setting up and running Linea services locally, with a specific focus on the coordinator service.

## Prerequisites

Before you start, make sure you have the following installed:

- Node.js v20 or higher
- Docker v24 or higher
  - Docker should have ~16 GB of Memory and 4+ CPUs to run the entire stack
- Docker Compose version v2.19+
- Make v3.81+
- Pnpm v9.14.4 (https://pnpm.io/installation)
- Java Development Kit (JDK) 21 (required for building the coordinator)
- Gradle 8.5+ (for building Java-based services)

## Building the Coordinator Locally

The coordinator is a Java-based service that orchestrates the Linea protocol's operations. You can build it locally using the following steps:

### 1. Clone the Repository

If you haven't already, clone the repository and navigate to the project directory:

```bash
git clone https://github.com/ConsenSys/linea-monorepo.git
cd linea-monorepo
```

### 2. Install Dependencies

Install the Node.js dependencies:

```bash
make pnpm-install
```

### 3. Build the Coordinator

The coordinator can be built using Gradle:

```bash
./gradlew :coordinator:app:build
```

This will generate the coordinator JAR file in the `coordinator/app/build/libs` directory.

### 4. Building the Coordinator Docker Image Locally

To build a local Docker image of the coordinator:

```bash
# First build the coordinator JAR
./gradlew coordinator:app:installDist

# Then build the Docker image
docker buildx build --file coordinator/Dockerfile --build-context libs=./coordinator/app/build/install/coordinator/lib --tag consensys/linea-coordinator:local .
```

## Running the Coordinator Locally

There are two main ways to run the coordinator:

### 1. Running as Part of the Full Stack

The recommended way to run the coordinator is as part of the complete Linea stack:

```bash
# Start the entire stack with tracing v2 using your local coordinator image
COORDINATOR_TAG=local make start-env-with-tracing-v2
```

This command:
1. Starts all necessary services (L1 node, L2 node, sequencer, etc.)
2. Deploys the required smart contracts
3. Starts the coordinator service with your locally built image

### 2. Running the Coordinator Standalone

If you want to run just the coordinator for development purposes, you can use:

```bash
# Set necessary environment variables
export POSTGRES_USER=postgres
export POSTGRES_PASSWORD=postgres

# Run only PostgreSQL database (required by coordinator)
docker compose -f docker/compose-tracing-v2.yml up -d postgres

# Run the coordinator with your local build
java -Dvertx.configurationFile=config/coordinator/vertx-options.json \
     -Dlog4j2.configurationFile=config/coordinator/log4j2-dev.xml \
     -jar coordinator/app/build/libs/coordinator.jar \
     --traces-limits-v2 config/common/traces-limits-v2.toml \
     --smart-contract-errors config/common/smart-contract-errors.toml \
     --gas-price-cap-time-of-day-multipliers config/common/gas-price-cap-time-of-day-multipliers.toml \
     config/coordinator/coordinator-config-v2.toml
```

Note: When running the coordinator standalone, you'll need to ensure that all its dependencies (such as L1 and L2 nodes) are properly configured and running.

## Configuration

The coordinator uses several configuration files:

- `config/coordinator/coordinator-config-v2.toml`: Main configuration file
- `config/common/traces-limits-v2.toml`: Traces limits configuration
- `config/common/smart-contract-errors.toml`: Smart contract errors configuration
- `config/common/gas-price-cap-time-of-day-multipliers.toml`: Gas price cap multipliers

For development, you may want to modify the following parameters in the coordinator configuration:

- `conflation.conflation-deadline`: Controls how long the coordinator waits before conflating blocks (default is 6 seconds)
- `l1-submission.blob.l1-endpoint`: Configure the connection to the L1 node for submitting blob tx (defaults to default.l1-endpoint)
- `l1-submission.aggregation.l1-endpoint`: Configure the connection to the L1 node for submitting aggregation tx (defaults to default.l1-endpoint)
- `conflation.l2-endpoint`: Configure the connection to the L2 node for conflation (defaults to default.l2-endpoint)
- `l1-finalization-monitor.l2-endpoint`: Configure the connection to the L2 node for retrieving L2 block data (defaults to default.l2-endpoint)
- `message-anchoring.l2-endpoint`: Configure the connection to the L2 node for submitting message anchoring tx (defaults to default.l2-endpoint)
- `prover` section: Configure the paths for prover requests and responses

## Troubleshooting

### Docker Issues

If you encounter issues with the Docker setup:

```bash
# Clean the environment and remove Docker volumes
make clean-environment
docker system prune --volumes
```

### Coordinator Connection Issues

If the coordinator has trouble connecting to other services:

1. Check that all required services are running:
   ```bash
   docker compose -f docker/compose-tracing-v2.yml ps
   ```

2. Verify network connectivity between containers:
   ```bash
   docker run --rm -it --network=docker_linea weibeld/ubuntu-networking bash
   ping coordinator
   ping postgres
   ping l1-el-node
   ```

3. Check the coordinator logs:
   ```bash
   docker logs coordinator
   ```

## Development Workflow

When developing the coordinator:

1. Make your code changes
2. Run tests: `./gradlew :coordinator:app:test`
3. Build the coordinator: `./gradlew :coordinator:app:build`
4. Build a local Docker image (as shown above)
5. Start the environment with your local image
6. Test your changes

## Relevant Endpoints

- Coordinator API: http://localhost:9545
- Sequencer RPC: http://localhost:8545
- Traces Node: http://localhost:8745
- PostgreSQL: localhost:5432
