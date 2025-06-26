# Linea Postman Service

The Linea Postman service is a component of the Linea blockchain infrastructure that facilitates cross-chain message delivery between Layer 1 (Ethereum) and Layer 2 (Linea).

## Overview

The Postman service monitors and processes messages between L1 and L2 chains, handling message submission, verification, and claiming. It operates as a Docker container and integrates with both L1 and L2 nodes.

It offers the following key features:

- Feature 1: Listening for message sent events on Ethereum and Linea
- Feature 2: Listening for message hash anchoring events to check if a message is ready to be claimed
- Feature 3: Automatic claiming of messages with a configurable retry mechanism
- Feature 4: Checking receipt status for each transaction

All messages are stored in a configurable Postgres DB.

## Configuration

### Environment Variables

#### L1 Configuration
- `L1_RPC_URL`: Ethereum node RPC endpoint
- `L1_CONTRACT_ADDRESS`: Address of the LineaRollup contract on L1
- `L1_SIGNER_PRIVATE_KEY`: Private key for L1 transactions
- `L1_LISTENER_INTERVAL`: Block listening interval (ms)
- `L1_LISTENER_INITIAL_FROM_BLOCK`: (optional) Starting block for event listening. This configuration option controls from which block the Postman service starts fetching events.
  - **Default behavior**: If not specified or set to `-1` (default), the service will start from the current latest block on the chain or from the latest processed block if there are previously processed messages in the database.
  - **Custom block number**: Set to a specific block number (e.g., `12345678`) to start fetching events from that block
  - **From genesis**: Set to `0` to fetch all historical events from the beginning of the chain
  - **Priority order**: The service determines the starting block using the following priority:
    1. If `L1_LISTENER_INITIAL_FROM_BLOCK` is set to a value greater than `-1`, use that block number
    2. If there are previously processed messages in the database, resume from the last processed block
    3. Otherwise, start from the current latest block on the chain
  - **⚠️ Performance Note**: Setting this to `0` or a very low block number may result in long initial sync times as the service will process all historical events
- `L1_LISTENER_BLOCK_CONFIRMATION`: Required block confirmations
- `L1_MAX_BLOCKS_TO_FETCH_LOGS`: Maximum blocks to fetch in one request
- `L1_MAX_GAS_FEE_ENFORCED`: Enable/disable gas fee enforcement
- `L1_EVENT_FILTER_FROM_ADDRESS`: Filter events using a from address
- `L1_EVENT_FILTER_TO_ADDRESS`: Filter events using a to address
- `L1_EVENT_FILTER_CALLDATA`: MessageSent event calldata filtering criteria expression. See [Filtrex repo](https://github.com/joewalnes/filtrex/tree/master).
    <br>
    You can filter by the calldata field:
    <br>

    Example:
    `calldata.funcSignature == "0x6463fb2a" and calldata.params.messageNumber == 85804`,
- `L1_EVENT_FILTER_CALLDATA_FUNCTION_INTERFACE`: Calldata data function interface following this format: `"function transfer(address to, uint256 amount)"`. Make sure you specify parameters names in order to use syntax like `calldata.params.messageNumber`.

#### L2 Configuration
- `L2_RPC_URL`: Linea node RPC endpoint
- `L2_CONTRACT_ADDRESS`: Address of the L2MessageService contract on L2
- `L2_SIGNER_PRIVATE_KEY`: Private key for L2 transactions
- `L2_LISTENER_INTERVAL`: Block listening interval (ms)
- `L2_LISTENER_INITIAL_FROM_BLOCK`: (optional) Starting block for event listening. This configuration option controls from which block the Postman service starts fetching events.
  - **Default behavior**: If not specified or set to `-1` (default), the service will start from the current latest block on the chain or from the latest processed block if there are previously processed messages in the database.
  - **Custom block number**: Set to a specific block number (e.g., `5432100`) to start fetching events from that block
  - **From genesis**: Set to `0` to fetch all historical events from the beginning of the chain
  - **Priority order**: The service determines the starting block using the following priority:
    1. If `L2_LISTENER_INITIAL_FROM_BLOCK` is set to a value greater than `-1`, use that block number
    2. If there are previously processed messages in the database, resume from the last processed block
    3. Otherwise, start from the current latest block on the chain
  - **⚠️ Performance Note**: Setting this to `0` or a very low block number may result in long initial sync times as the service will process all historical events
- `L2_LISTENER_BLOCK_CONFIRMATION`: Required block confirmations
- `L2_MAX_BLOCKS_TO_FETCH_LOGS`: Maximum blocks to fetch in one request
- `L2_MAX_GAS_FEE_ENFORCED`: Enable/disable gas fee enforcement
- `L2_MESSAGE_TREE_DEPTH`: Depth of the message Merkle tree
- `L2_EVENT_FILTER_FROM_ADDRESS`: Filter events using a from address
- `L2_EVENT_FILTER_TO_ADDRESS`: Filter events using a to address
- `L2_EVENT_FILTER_CALLDATA`: MessageSent event calldata filtering criteria expression. See [Filtrex repo](https://github.com/joewalnes/filtrex/tree/master).
    <br>
    You can filter by the calldata field:
    <br>

    Example:
    `calldata.funcSignature == "0x6463fb2a" and calldata.params.messageNumber == 85804`,
- `L2_EVENT_FILTER_CALLDATA_FUNCTION_INTERFACE`: Calldata data function interface following this format: `"function transfer(address to, uint256 amount)"`. Make sure you specify parameters names in order to use syntax like `calldata.params.messageNumber`.

#### Message Processing
- `MESSAGE_SUBMISSION_TIMEOUT`: Timeout for message submission (ms)
- `MAX_FETCH_MESSAGES_FROM_DB`: Maximum messages to fetch from database
- `MAX_NONCE_DIFF`: Maximum allowed nonce difference between the DB and the chain
- `MAX_FEE_PER_GAS_CAP`: Maximum gas fee cap
- `GAS_ESTIMATION_PERCENTILE`: Gas estimation percentile
- `PROFIT_MARGIN`: Profit margin for gas fees
- `MAX_NUMBER_OF_RETRIES`: Maximum retry attempts
- `RETRY_DELAY_IN_SECONDS`: Delay between retries
- `MAX_CLAIM_GAS_LIMIT`: Maximum gas limit for claim transactions
- `MAX_POSTMAN_SPONSOR_GAS_LIMIT`: Maximum gas limit for sponsored Postman claim transactions

#### Feature Flags
- `L1_L2_EOA_ENABLED`: Enable L1->L2 EOA messages
- `L1_L2_CALLDATA_ENABLED`: Enable L1->L2 calldata messages
- `L1_L2_AUTO_CLAIM_ENABLED`: Enable auto-claiming for L1->L2 messages
- `L2_L1_EOA_ENABLED`: Enable L2->L1 EOA messages
- `L2_L1_CALLDATA_ENABLED`: Enable L2->L1 calldata messages
- `L2_L1_AUTO_CLAIM_ENABLED`: Enable auto-claiming for L2->L1 messages
- `ENABLE_LINEA_ESTIMATE_GAS`: Enable `linea_estimateGas`endpoint usage for L2 chain gas fees estimation
- `DB_CLEANER_ENABLED`: Enable DB cleaning to delete old claimed messages
- `L1_L2_ENABLE_POSTMAN_SPONSORING`: Enable L1->L2 Postman sponsoring for claiming messages
- `L2_L1_ENABLE_POSTMAN_SPONSORING`: Enable L2->L1 Postman sponsoring for claiming messages

#### DB cleaning
- `DB_CLEANING_INTERVAL`: DB cleaning polling interval (ms)
- `DB_DAYS_BEFORE_NOW_TO_DELETE`: Number of days to retain messages in the database before deletion. Messages older than this number of days will be automatically cleaned up if they are in a final state (CLAIMED_SUCCESS, CLAIMED_REVERTED, EXCLUDED, or ZERO_FEE)

#### Database Configuration
- `POSTGRES_HOST`: PostgreSQL host
- `POSTGRES_PORT`: PostgreSQL port
- `POSTGRES_USER`: Database user
- `POSTGRES_PASSWORD`: Database password
- `POSTGRES_DB`: Database name

### Starting Block Configuration Examples

The `L1_LISTENER_INITIAL_FROM_BLOCK` and `L2_LISTENER_INITIAL_FROM_BLOCK` configuration options are crucial for controlling event fetching behavior. Here are practical examples:

#### Example 1: Development Setup (Default Behavior)
```bash
# For development, start from current block to avoid processing historical data
# These variables are optional - if not specified, default value -1 will be used
L1_LISTENER_INITIAL_FROM_BLOCK=-1  # Default value (optional)
L2_LISTENER_INITIAL_FROM_BLOCK=-1  # Default value (optional)
```

#### Example 2: Fresh Production Deployment from Genesis
```bash
# Process all historical events from the beginning of the chains
L1_LISTENER_INITIAL_FROM_BLOCK=0
L2_LISTENER_INITIAL_FROM_BLOCK=0
```

#### Example 3: Production Deployment from Specific Block Heights
```bash
# Start from specific block numbers to avoid processing unnecessary historical data
L1_LISTENER_INITIAL_FROM_BLOCK=18500000  # Ethereum block number
L2_LISTENER_INITIAL_FROM_BLOCK=1000000   # Linea block number
```

#### Example 4: Recovery from Known Good State
```bash
# After database restoration, resume from known block heights
L1_LISTENER_INITIAL_FROM_BLOCK=18550000
L2_LISTENER_INITIAL_FROM_BLOCK=1050000
```

#### Use Case Guidelines:

- **New Development Environment**: Use default values (`-1`) to start fresh from current block
- **Production Mainnet**: Set to specific block numbers where message bridge was deployed or activated
- **Testing Historical Data**: Set to `0` or specific historical blocks (expect longer sync times)
- **Database Recovery**: Set to last known good block numbers after database restoration
- **Performance Optimization**: Use higher block numbers to skip irrelevant historical events

**Note**: The `L1_LISTENER_INITIAL_FROM_BLOCK` and `L2_LISTENER_INITIAL_FROM_BLOCK` configuration values are always prioritized when set to a value greater than -1, even if the database contains previously processed messages.

## Development

### Running

#### Start the docker local stack

From the root folder, run the following command:
```bash
make start-env-with-tracing-v2
```

Stop the postman docker container manually.

#### Run the postman locally:

Before the postman can be run and tested locally, we must build the monorepo projects linea-sdk and linea-native-libs
```bash
NATIVE_LIBS_RELEASE_TAG=blob-libs-v1.2.0 pnpm run -F linea-native-libs build && pnpm run -F linea-sdk build
```

From the postman folder run the following commands:

```bash
# Create a new .env file
cp .env.sample .env

# Run the postman
ts-node scripts/runPostman.ts
```

### Building
```bash
# Build the Postman service
pnpm run build
```

### Testing

```bash
# Run unit tests
pnpm run test
```

### Database migrations

Create an empty DB migration file with <NAME>

```bash
MIGRATION_NAME=<NAME> pnpm run migration:create
```

We will then implement the migration code manually. We omit scripts for TypeORM migration generation because the CLI tool is unable to generate correct migration code in our case.

## License

This package is licensed under the [Apache 2.0](../LICENSE-APACHE) and the [MIT](../LICENSE-MIT) licenses.
