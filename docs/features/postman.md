# Postman

> Automated cross-chain message relay service for claiming messages between L1 and L2.

## Overview

The Postman is a TypeScript/Express service that automates the claiming step of Linea's messaging protocol. When a user sends a cross-chain message, someone must call `claimMessage` (or `claimMessageWithProof`) on the destination chain. The Postman monitors anchored messages and submits claim transactions automatically.

## Message Lifecycle

Messages progress through a state machine tracked in PostgreSQL:

```
SENT → ANCHORED → TRANSACTION_SIZE_COMPUTED* → PENDING → CLAIMED_SUCCESS
                                                      └→ CLAIMED_REVERTED
                                                      └→ NON_EXECUTABLE
                                                      └→ ZERO_FEE
                                                      └→ FEE_UNDERPRICED
       └→ EXCLUDED
```

\* `TRANSACTION_SIZE_COMPUTED` applies only to L1→L2 messages (see below).

| Status | Meaning |
|--------|---------|
| `SENT` | Event detected on source chain |
| `ANCHORED` | Message claimable on destination chain |
| `TRANSACTION_SIZE_COMPUTED` | (L1→L2 only) Compressed tx size calculated for gas estimation |
| `PENDING` | Claim transaction submitted |
| `CLAIMED_SUCCESS` | Claim confirmed |
| `CLAIMED_REVERTED` | Claim reverted (may retry if rate-limited) |
| `NON_EXECUTABLE` | Gas estimation failed or exceeds cap; will not retry |
| `ZERO_FEE` | No fee attached (see Sponsorship) |
| `FEE_UNDERPRICED` | Fee below estimated gas cost × profit margin |
| `EXCLUDED` | Filtered out by event/EOA rules |

## Sponsorship Model

By default, the Postman only claims messages where the attached fee covers gas plus a profit margin (`PROFIT_MARGIN`). Messages below this threshold are marked `ZERO_FEE` or `FEE_UNDERPRICED`.

When sponsorship is enabled (`L1_L2_ENABLE_POSTMAN_SPONSORING` / `L2_L1_ENABLE_POSTMAN_SPONSORING`), the Postman pays gas from its own funds:

| Condition | Without Sponsorship | With Sponsorship |
|-----------|---------------------|------------------|
| `fee = 0` | `ZERO_FEE` (terminal) | Claimed if gas ≤ `MAX_POSTMAN_SPONSOR_GAS_LIMIT` |
| `fee < gasCost` | `FEE_UNDERPRICED` (retry later) | Claimed if gas ≤ `MAX_POSTMAN_SPONSOR_GAS_LIMIT` |
| `fee ≥ gasCost` | Claimed normally | Claimed normally |

## L1→L2 vs L2→L1 Differences

**L1→L2** includes an extra `TRANSACTION_SIZE_COMPUTED` step. Linea's variable gas pricing depends on compressed transaction size, so the Postman pre-computes this via `@consensys/linea-native-libs` before estimating gas.

**L2→L1** uses standard EIP-1559 gas estimation and requires Merkle proofs (retrieved from Shomei) for claiming on L1.

## Retry Logic

- **Rate limit errors**: Claim reverts with `RateLimitExceeded` → message resets to `SENT` and retries later
- **`FEE_UNDERPRICED`**: Re-evaluated periodically; becomes claimable if gas prices drop
- **Transaction timeout**: `PENDING` transactions not mined within `MESSAGE_SUBMISSION_TIMEOUT` are retried with higher gas (up to `MAX_TX_RETRIES`)

## Components

| Component | Path | Role |
|-----------|------|------|
| PostmanServiceClient | `postman/src/application/postman/app/` | Main application orchestrator |
| Event Pollers | `postman/src/services/pollers/` | Poll `MessageSent` events on both chains |
| Anchoring Processor | `postman/src/services/processors/` | Check message anchoring status |
| Claiming Processor | `postman/src/services/processors/` | Submit claim transactions |
| L2ClaimMessageTransactionSizeProcessor | `postman/src/services/processors/` | Compressed tx size calculation (L1→L2) |
| DatabaseCleaningPoller | `postman/src/services/pollers/` | Periodic cleanup of old records |

## Test Coverage

| Test File | Runner | Validates |
|-----------|--------|-----------|
| `postman/src/**/__tests__/` | Jest | Pollers, processors, service logic |
| `e2e/src/messaging.spec.ts` | Jest | Full L1↔L2 message round-trips with Postman sponsoring |

## Related Documentation

- [Messaging Feature](messaging.md) — Message protocol overview
- [Tech: Postman Component](../tech/components/postman.md) — Database schema, full config reference, directory structure, build/run instructions
- [Official docs: Canonical Message Service](https://docs.linea.build/protocol/architecture/interoperability/canonical-message-service)
