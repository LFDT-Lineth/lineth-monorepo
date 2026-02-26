# Operations Guide

## Operational Tools

The monorepo includes several operational utilities for managing the Linea network.

## Operations CLI (`operations/`)

Command-line tools for network operations.

### Available Commands

```bash
cd operations

# Build first (required)
pnpm run build

# ETH Transfer (validator rewards)
./bin/run.js eth-transfer \
  --config ./config.json \
  --dry-run

# Submit Invoice
./bin/run.js submit-invoice \
  --config ./config.json

# Burn and Bridge (LINEA token)
./bin/run.js burn-and-bridge \
  --config ./config.json

# Sync Transactions
./bin/run.js synctx \
  --config ./config.json
```

> **Note**: The operations CLI uses [oclif](https://oclif.io/). Run `./bin/run.js --help` to see all available commands.

### ETH Transfer

Distributes validator rewards.

```typescript
// Configuration
{
  "rpcUrl": "https://...",
  "privateKey": "0x...",
  "recipients": [
    { "address": "0x...", "amount": "1.5" }
  ]
}
```

### Submit Invoice

Submits invoices to on-chain contract.

### Burn and Bridge

Burns LINEA tokens and bridges to L2.

## Native Yield Operations (`native-yield-operations/`)

Automation service for native yield management.

### Purpose

- Monitoring L1MessageService balance and rebalancing according to maintain reserve thresholds
- Monitor staking vault balances
- Report yield to YieldManager contract
- Handle ossification states
- Manage validator operations

### Architecture

```
┌──────────────────────────────────────────────────────────────────────┐
│                     Native Yield Automation                          │
│                                                                      │
│  ┌────────────────────────────────────────────────────────────────┐  │
│  │                   Operation Mode Selector                      │  │
│  │                                                                │  │
│  │       Determines current mode based on YieldProvider ossification state          │  │
│  └───────────────────────────┬────────────────────────────────────┘  │
│                              │                                       │
│         ┌────────────────────┼────────────────────┐                  │
│         │                    │                    │                  │
│         ▼                    ▼                    ▼                  │
│  ┌─────────────┐      ┌─────────────┐      ┌─────────────┐           │
│  │   Yield     │      │ Ossification│      │ Ossification│           │
│  │  Reporting  │      │   Pending   │      │  Complete   │           │
│  │  Processor  │      │  Processor  │      │  Processor  │           │
│  └─────────────┘      └─────────────┘      └─────────────┘           │
│                                                                      │
└──────────────────────────────────────────────────────────────────────┘
```

> **Diagram:** [Native Yield Automation](../diagrams/native-yield-automation.mmd) (Mermaid source)

### Running

```bash
cd native-yield-operations/automation-service

# Install dependencies
pnpm install

# Configure environment
cp .env.sample .env
# Edit .env with your configuration

# Start service
pnpm run start
```

> **Configuration**: See the [automation-service README](../../native-yield-operations/automation-service/README.md#configuration) for environment variable details.

## Contract Verification

### Contract Integrity Verifier (`contract-integrity-verifier/`)

Verifies deployed contract bytecode matches source.

```bash
cd contract-integrity-verifier

# Install
pnpm install

# Verify contract
pnpm run verify \
  --network mainnet \
  --address 0x... \
  --expected-bytecode ./artifacts/Contract.json
```

### Packages

- `verifier-core`: Core verification logic
- `verifier-viem`: Viem-based implementation
- `verifier-ethers`: Ethers-based implementation

## State Recovery

### Starting Recovery Stack

```bash
make start-env-with-staterecovery
```

### Replaying from Block

```bash
make staterecovery-replay-from-block \
  L1_ROLLUP_CONTRACT_ADDRESS=0x... \
  STATERECOVERY_OVERRIDE_START_BLOCK_NUMBER=1
```

### Recovery Process

1. **Monitor L1**: Watch for `DataSubmittedV3` events
2. **Fetch Blobs**: Get blob data from BlobScan
3. **Decompress**: Decompress blob data
4. **Import**: Import blocks into Besu
5. **Verify**: Check state root matches

## Deployment Checklist

### Pre-deployment

- [ ] All tests passing
- [ ] Contract bytecode verified
- [ ] Environment variables configured
- [ ] RPC endpoints accessible
- [ ] Wallet funded with ETH

### Deployment Steps

1. Deploy PlonkVerifier
2. Deploy LineaRollup (with verifier address)
3. Deploy L2MessageService
4. Deploy TokenBridge (L1 and L2)
5. Verify all contract deployments
6. Initialize contracts (set operators, etc.)

### Post-deployment

- [ ] Verify contract on Etherscan/Lineascan
- [ ] Test basic operations
- [ ] Monitor for errors
- [ ] Update documentation

## Rollback Procedures

### Coordinator Issues

1. Stop coordinator: `docker stop coordinator`
2. Check logs: `docker logs coordinator`
3. Restore from backup if needed
4. Restart: `docker start coordinator`

### Database Recovery

```bash
# Connect to PostgreSQL
docker exec -it postgres psql -U postgres

# Check tables
\dt

# Backup
pg_dump -U postgres coordinator > backup.sql

# Restore
psql -U postgres coordinator < backup.sql
```

### Contract Emergency

For contract emergencies:
1. Pause affected functionality
2. Investigate root cause
3. Prepare fix or rollback
4. Execute through timelock (if applicable)

## Security Considerations

### Key Management

- Never commit private keys
- Use environment variables or secret management
- Rotate keys periodically
- Use hardware wallets for production

### Access Control

- Limit who can deploy contracts
- Use multi-sig for admin operations
- Audit all contract changes

### Monitoring

- Set up alerts for anomalies
- Monitor gas prices
- Track finalization delays
- Watch for reorgs

## Useful Scripts

### Check Service Health

```bash
#!/bin/bash
services=("sequencer" "coordinator" "l1-el-node")
for svc in "${services[@]}"; do
  status=$(docker inspect --format='{{.State.Health.Status}}' "$svc" 2>/dev/null)
  echo "$svc: ${status:-not running}"
done
```

### Tail All Logs

```bash
#!/bin/bash
docker compose -f docker/compose-tracing-v2.yml logs -f \
  coordinator sequencer prover-v3 postman
```

### Get Current Finalized Block

```bash
#!/bin/bash
curl -s -X POST http://localhost:8545 \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"eth_getBlockByNumber","params":["finalized",false],"id":1}' \
  | jq -r '.result.number' | xargs printf "%d\n"
```
