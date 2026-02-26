# Token Generation Event (TGE)

> LINEA token contracts (L1/L2), cross-chain supply sync, and token airdrops.

## Overview

The TGE module defines the LINEA token on both L1 and L2, plus an airdrop mechanism. The L1 token mints the initial supply and syncs total supply to L2 via the message service. The L2 token integrates with the canonical token bridge for mint/burn and supports ERC20Votes for governance.

Located in `contracts/token-generation-event/` as a separate Foundry sub-project.

## Components

| Component | Path | Role |
|-----------|------|------|
| LineaToken | `contracts/token-generation-event/src/L1/LineaToken.sol` | L1 LINEA token (ERC20, Burnable, Permit, AccessControl) |
| L2LineaToken | `contracts/token-generation-event/src/L2/L2LineaToken.sol` | L2 LINEA token (ERC20, Permit, Votes, AccessControl) |
| MessageServiceBase | `contracts/token-generation-event/src/MessageServiceBase.sol` | Cross-chain message authentication |
| TokenAirdrop | `contracts/token-generation-event/src/airdrops/TokenAirdrop.sol` | Merkle-free allocation-based airdrop |
| Reference Tokens | `contracts/token-generation-event/src/referenceTokens/` | Soulbound multiplier tokens for airdrop calculations |

## L1 LineaToken

Upgradeable ERC20 with:
- `MINTER_ROLE` for initial supply minting
- `syncTotalSupplyToL2()` sends total supply to L2 via the L1 message service
- Burnable, Permit (ERC-2612)

## L2 LineaToken

Upgradeable ERC20 with:
- `mint` / `burn` restricted to `lineaCanonicalTokenBridge`
- `syncTotalSupplyFromL1()` — callable only from L2 message service with `remoteSender == l1TokenAddress`
- ERC20Votes for governance delegation
- Permit (ERC-2612)

## Cross-Chain Supply Sync

```mermaid
sequenceDiagram
    participant L1Token as LineaToken (L1)
    participant MS_L1 as L1MessageService
    participant MS_L2 as L2MessageService
    participant L2Token as L2LineaToken

    L1Token->>MS_L1: sendMessage(L2Token, syncTotalSupplyFromL1 calldata)
    Note over MS_L1,MS_L2: Message anchored via coordinator
    MS_L2->>L2Token: syncTotalSupplyFromL1(totalSupply)
    L2Token->>L2Token: Update cached total supply
```

## TokenAirdrop

Allocation-based airdrop using primary and secondary factors with configurable multipliers:
- `calculateAllocation(address)` — Computes allocation from primary/secondary factors and multiplier tokens
- `claim()` — Claims allocated tokens (one-time per address)
- `DENOMINATOR = 1e9` for basis-point precision
- Reference tokens (`PrimaryExplicitMultiplier`, `SecondaryExplicitMultiplier`, `PrimaryConditionalMultiplier`) are soulbound ERC20s — transfers and approvals revert.

## Test Coverage

| Test File | Runner | Validates |
|-----------|--------|-----------|
| `contracts/token-generation-event/test/L1/LineaToken.t.sol` | Foundry | L1 token init, mint, burn, supply sync |
| `contracts/token-generation-event/test/L2/L2LineaToken.t.sol` | Foundry | L2 token init, bridge mint/burn, votes |
| `contracts/token-generation-event/test/airdrops/TokenAirdrop.t.sol` | Foundry | Allocation calculation, claiming, withdrawal |

## Related Documentation

- [Messaging Feature](messaging.md) — Cross-chain message protocol used by supply sync
- [Token Bridge Feature](token-bridge.md) — Canonical bridge used by L2 token mint/burn
