# Pause and Security

> Type-based pausing, rate limiting, role-based access control, and reentrancy protection.

## Overview

Linea's security model combines three independent mechanisms enforced at the smart contract layer:

1. **Type-based pausing** — Granular pause/unpause per operation type with time-based expiry and security council override.
2. **Rate limiting** — Period-based ETH throughput caps on messaging operations.
3. **Access control** — OpenZeppelin `AccessControl` with per-contract role definitions.

## Pause System

### PauseManager (`contracts/src/security/pausing/PauseManager.sol`)

Pauses are scoped by `PauseType` enum (defined in `IPauseManager.sol`):

| PauseType | Affected Operations |
|-----------|-------------------|
| `UNUSED` | Placeholder (value 0) |
| `GENERAL` | All operations on the contract |
| `L1_L2` | L1→L2 `sendMessage` |
| `L2_L1` | L2→L1 `sendMessage` |
| `BLOB_SUBMISSION` | *(deprecated)* |
| `CALLDATA_SUBMISSION` | *(deprecated)* |
| `FINALIZATION` | `finalizeBlocks` |
| `INITIATE_TOKEN_BRIDGING` | `bridgeToken` / `bridgeTokenWithPermit` |
| `COMPLETE_TOKEN_BRIDGING` | `completeBridging` on destination chain |
| `NATIVE_YIELD_STAKING` | Yield provider staking |
| `NATIVE_YIELD_UNSTAKING` | Yield provider unstaking |
| `NATIVE_YIELD_PERMISSIONLESS_ACTIONS` | Permissionless unstaking / reserve replenishment |
| `NATIVE_YIELD_REPORTING` | Yield reporting to L2 |
| `STATE_DATA_SUBMISSION` | `submitBlobs` / `submitDataAsCalldata` |

### Pause Mechanics

- **Duration**: Each pause lasts `PAUSE_DURATION` (48 hours) then auto-expires.
- **Cooldown**: After expiry, a `COOLDOWN_DURATION` (48 hours) prevents re-pausing by the same role.
- **Security council**: Holders of `SECURITY_COUNCIL_ROLE` can pause indefinitely (no expiry). Only the security council can unpause an indefinite pause.
- **Cooldown reset**: Security council can call `resetNonSecurityCouncilCooldownEnd()` to allow earlier re-pausing.

### Pause Roles

Each `PauseType` has independently configurable pause and unpause roles set at initialization. `PAUSE_ALL_ROLE` and `UNPAUSE_ALL_ROLE` apply to all types.

### Contract-Specific Pause Managers

Each major contract has a specialized pause manager in `contracts/src/security/pausing/`:
- `LineaRollupPauseManager.sol`
- `L2MessageServicePauseManager.sol`
- `TokenBridgePauseManager.sol`
- `YieldManagerPauseManager.sol`

### Key Functions

```solidity
function pauseByType(PauseType _pauseType) external;
function unPauseByType(PauseType _pauseType) external;
function unPauseByExpiredType(PauseType _pauseType) external;
function isPaused(PauseType _pauseType) public view returns (bool);
function updatePauseTypeRole(PauseType _pauseType, bytes32 _newRole) external;
function resetNonSecurityCouncilCooldownEnd() external onlyRole(SECURITY_COUNCIL_ROLE);
```

## Rate Limiting

### RateLimiter (`contracts/src/security/limiting/RateLimiter.sol`)

Limits the total ETH value flowing through messaging operations within a configurable time window.

| Parameter | Description |
|-----------|-------------|
| `periodInSeconds` | Rolling window duration |
| `limitInWei` | Max ETH throughput per period |
| `currentPeriodEnd` | Timestamp when current period expires |
| `currentPeriodAmountInWei` | ETH consumed in current period |

### Roles

| Role | Purpose |
|------|---------|
| `RATE_LIMIT_SETTER_ROLE` | Change `periodInSeconds` and `limitInWei` |
| `USED_RATE_LIMIT_RESETTER_ROLE` | Reset `currentPeriodAmountInWei` to zero |

### Key Functions

```solidity
function resetRateLimitAmount(uint256 _amount) external onlyRole(RATE_LIMIT_SETTER_ROLE);
function resetAmountUsedInPeriod() external onlyRole(USED_RATE_LIMIT_RESETTER_ROLE);
```

Internally, `_addUsedAmount(uint256)` is called by messaging functions and reverts with `RateLimitExceeded` if the period cap is breached.

## Access Control

### PermissionsManager (`contracts/src/security/access/PermissionsManager.sol`)

Wraps OpenZeppelin `AccessControlUpgradeable` for batch role assignment at initialization. Roles are granted via `RoleAddress[]` arrays during `initialize` or `reinitialize` calls.

### Reentrancy Protection

`TransientStorageReentrancyGuardUpgradeable` (`contracts/src/security/reentrancy/`) uses EIP-1153 transient storage for gas-efficient reentrancy guards.

---

## Governance and Upgrades

### Timelock

`TimeLock` (`contracts/src/governance/TimeLock.sol`) wraps OpenZeppelin `TimelockController` for time-delayed execution of privileged operations (upgrades, role changes).

### Upgrade Process

All core contracts follow the same upgrade pattern:

1. Deploy new implementation contract
2. Schedule upgrade via Timelock (multisig proposal)
3. After delay, execute `upgradeAndCall` on the proxy
4. New implementation runs `reinitializeVN()` with incremented version
5. `InitializationVersionCheck.onlyInitializedVersion(N-1)` ensures sequential upgrades

Each contract tracks its own `_CONTRACT_VERSION` string and reinitializer version. Check the respective source for current values.

### Security Council

The `SECURITY_COUNCIL_ROLE` grants:
- Indefinite pausing (no 48-hour expiry)
- Unpause of indefinite pauses
- Cooldown reset for non-council pause roles

See [Security Council Charter](../../contracts/docs/security-council-charter.md) for operational details.

## Test Coverage

| Test File | Runner | Validates |
|-----------|--------|-----------|
| `contracts/test/hardhat/security/PauseManager.ts` | Hardhat | Pause/unpause, roles, expiry, cooldown, security council |
| `contracts/test/hardhat/security/RateLimiter.ts` | Hardhat | Period/limit config, overflow, reset |
| `contracts/test/hardhat/governance/Timelock.ts` | Hardhat | Timelock scheduling, execution, cancellation |

## Related Documentation

- [Tech: Contracts Component](../tech/components/contracts.md) — Security features, access control roles, upgrade pattern
- [Workflow: Pausing](../../contracts/docs/workflows/administration/pausing.md)
- [Workflow: Unpausing](../../contracts/docs/workflows/administration/unpausing.md)
- [Workflow: Rate Limiting](../../contracts/docs/workflows/administration/rateLimiting.md)
- [Workflow: Role Management](../../contracts/docs/workflows/administration/roleManagement.md)
- [Workflow: Upgrade Contract](../../contracts/docs/workflows/administration/upgradeContract.md)
- [Workflow: Upgrade and Call](../../contracts/docs/workflows/administration/upgradeAndCallContract.md)
- [Security Council Charter](../../contracts/docs/security-council-charter.md)
