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
- `L1_LISTENER_INITIAL_FROM_BLOCK`: Starting block for event listening (optional)
- `L1_LISTENER_BLOCK_CONFIRMATION`: Required block confirmations
- `L1_MAX_BLOCKS_TO_FETCH_LOGS`: Maximum blocks to fetch in one request
- `L1_MAX_GAS_FEE_ENFORCED`: Enable/disable gas fee enforcement

#### L2 Configuration
- `L2_RPC_URL`: Linea node RPC endpoint
- `L2_CONTRACT_ADDRESS`: Address of the L2MessageService contract on L2
- `L2_SIGNER_PRIVATE_KEY`: Private key for L2 transactions
- `L2_LISTENER_INTERVAL`: Block listening interval (ms)
- `L2_LISTENER_INITIAL_FROM_BLOCK`: Starting block for event listening (optional)
- `L2_LISTENER_BLOCK_CONFIRMATION`: Required block confirmations
- `L2_MAX_BLOCKS_TO_FETCH_LOGS`: Maximum blocks to fetch in one request
- `L2_MAX_GAS_FEE_ENFORCED`: Enable/disable gas fee enforcement
- `L2_MESSAGE_TREE_DEPTH`: Depth of the message Merkle tree

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

#### Feature Flags
- `L1_L2_EOA_ENABLED`: Enable L1->L2 EOA messages
- `L1_L2_CALLDATA_ENABLED`: Enable L1->L2 calldata messages
- `L1_L2_AUTO_CLAIM_ENABLED`: Enable auto-claiming for L1->L2 messages
- `L2_L1_EOA_ENABLED`: Enable L2->L1 EOA messages
- `L2_L1_CALLDATA_ENABLED`: Enable L2->L1 calldata messages
- `L2_L1_AUTO_CLAIM_ENABLED`: Enable auto-claiming for L2->L1 messages
- `ENABLE_LINEA_ESTIMATE_GAS`: Enable `linea_estimateGas`endpoint usage for L2 chain gas fees estimation
- `DB_CLEANER_ENABLED`: Enable DB cleaning to delete old claimed messages

#### DB cleaning
- `DB_CLEANING_INTERVAL`: DB cleaning polling interval (ms)
- `DB_DAYS_BEFORE_NOW_TO_DELETE`: Number of days to retain messages in the database before deletion. Messages older than this number of days will be automatically cleaned up if they are in a final state (CLAIMED_SUCCESS, CLAIMED_REVERTED, EXCLUDED, or ZERO_FEE)

#### Database Configuration
- `POSTGRES_HOST`: PostgreSQL host
- `POSTGRES_PORT`: PostgreSQL port
- `POSTGRES_USER`: Database user
- `POSTGRES_PASSWORD`: Database password
- `POSTGRES_DB`: Database name

## Development

### Running

#### Start the docker local stack

From the root folder, run the following command:
```bash
make fresh-start-all
```

Stop the postman docker container manually.

#### Run the postman locally:

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

## License

This package is licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE) for more information.
