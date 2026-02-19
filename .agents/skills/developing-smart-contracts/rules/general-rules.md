# General Rules

**Impact: MEDIUM (improves code quality and maintainability)**

## Inheritance & Customization

When extending Linea contracts:

- Use `virtual`/`override` keywords
- Override `CONTRACT_VERSION()` for custom versions
- See examples in `src/_testing/unit/` for patterns

```solidity
// Base contract
function getVersion() public pure virtual returns (string memory) {
  return "1.0.0";
}

// Extended contract
function getVersion() public pure override returns (string memory) {
  return "1.1.0";
}
```

### `public` vs `external` for Overridden Functions

Use `public virtual` for overridable functions. `public` allows children to both replace and extend (via `super`), whereas `external virtual` only allows replacement. Ensure functions that can be called internally have reentrancy guards. Avoid making functions `public virtual` if they check state before modifying it (TOCTOU risk - the state may change between the check and the use).

```solidity
// Base contract: public virtual allows children to replace or extend
function abc() public virtual nonReentrant {
  // base logic
}

// Child contract: safe because nonReentrant protects the internal call path
function abc() public override nonReentrant {
  super.abc();
  // additional logic
}
```

Reserve `external` for functions that are not intended to be overridden.

## OpenZeppelin Dependencies

Use OpenZeppelin contracts version **4.9.6** for both standard and upgradeable contracts:

```json
"@openzeppelin/contracts": "4.9.6",
"@openzeppelin/contracts-upgradeable": "4.9.6"
```

### Import Examples

```solidity
import { Ownable } from "@openzeppelin/contracts/access/Ownable.sol";
import { OwnableUpgradeable } from "@openzeppelin/contracts-upgradeable/access/OwnableUpgradeable.sol";
import { SafeERC20, IERC20 } from "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";
```

## Explicit State Variable Visibility

Always declare visibility on state variables. Solidity defaults to `internal`, but relying on the default has been flagged in audits as unclear intent.

```solidity
// Correct: explicit visibility
uint256 internal constant MAX_BPS = 10000;

// Incorrect: relies on default internal visibility
uint256 constant MAX_BPS = 10000;
```

## Zero-Value Checks

Use `ErrorUtils` from `src/libraries/ErrorUtils.sol` for zero-address and zero-hash checks. Do not write inline comparisons.

```solidity
import { ErrorUtils } from "../libraries/ErrorUtils.sol";

// Correct
ErrorUtils.revertIfZeroAddress(_addr);
ErrorUtils.revertIfZeroHash(_hash);

// Incorrect
if (_addr == address(0)) revert ZeroAddressNotAllowed();
require(_hash != bytes32(0), "zero hash");
```

## Avoid Magic Numbers

Use named constants instead of hardcoded values:

```solidity
// Incorrect: magic numbers
function withdraw() external {
  require(block.timestamp > 1704067200, "Too early");
  require(amount <= 1000000000000000000, "Too much");
}

// Correct: named constants
uint256 public constant WITHDRAWAL_START = 1704067200;
uint256 public constant MAX_WITHDRAWAL = 1 ether;

function withdraw() external {
  require(block.timestamp > WITHDRAWAL_START, "Too early");
  require(amount <= MAX_WITHDRAWAL, "Too much");
}
```

## Reinitializer Pattern

Both `initialize` (for new chain deployments) and `reinitializeVN` (for canonical chain upgrades) MUST use the same `reinitializer(N)` modifier. Do not use the `initializer` modifier.

```solidity
// Correct: both functions use reinitializer(8) and `initialize` also uses `onlyInitializedVersion(0)` for new deployments
function initialize(...) external onlyInitializedVersion(0) reinitializer(8) { ... }
function reinitializeV8(...) external reinitializer(8) { ... }

// Incorrect: initialize uses initializer
function initialize(...) external initializer { ... }
function reinitializeV8(...) external reinitializer(8) { ... }
```

Why:
- `onlyInitializedVersion(0)` enforces the contract is not accidentally being reinitialized from a lower version
- `_getInitializedVersion()` returns the version number even on fresh deployments, giving a consistent version indicator
- No access control or proxy admin needed on the implementation contract - `reinitializeV8` naturally fails on new deployments of this version
- In `reinitializeVN`, you can check `_getInitializedVersion() == N-1` to prevent upgrades from the wrong version

## Extract Repeated Checks into Modifiers

When the same conditional check appears in multiple functions, extract it into a modifier.

```solidity
// Correct: modifier for repeated check
modifier onlyWhenWithdrawalReserveInDeficit(uint256 _amountToSubtract) {
  if (!_isWithdrawalReserveBelowMinimum(_amountToSubtract)) revert WithdrawalReserveNotInDeficit();
  _;
}

function unstakePermissionless(...) external onlyWhenWithdrawalReserveInDeficit(msg.value) { ... }
function replenishWithdrawalReserve(...) external onlyWhenWithdrawalReserveInDeficit(0) { ... }

// Incorrect: duplicating the same check inline
function unstakePermissionless(...) external {
  if (!_isWithdrawalReserveBelowMinimum(msg.value)) revert WithdrawalReserveNotInDeficit();
  ...
}
function replenishWithdrawalReserve(...) external {
  if (!_isWithdrawalReserveBelowMinimum(0)) revert WithdrawalReserveNotInDeficit();
  ...
}
```

## Namespaced Storage (ERC-7201)

For new upgradeable contracts, prefer ERC-7201 namespaced storage over storage gaps. Storage gaps require manual size updates each time a new variable is added and are prone to human error.

Pattern (from `LineaRollupYieldExtension.sol`):

```solidity
/// @custom:storage-location erc7201:linea.storage.ContractNameStorage
struct ContractNameStorage {
  address _someVar;
}

// keccak256(abi.encode(uint256(keccak256("linea.storage.ContractNameStorage")) - 1)) & ~bytes32(uint256(0xff))
bytes32 private constant ContractNameStorageLocation =
  0x...; // precomputed slot

function _storage() private pure returns (ContractNameStorage storage $) {
  assembly {
    $.slot := ContractNameStorageLocation
  }
}
```

Key points:
- Annotate with `@custom:storage-location erc7201:linea.storage.ContractNameStorage`
- Wrap all storage variables in a single struct
- Expose a `_storage()` accessor that loads the struct from the computed slot
- Compute the slot via `keccak256(abi.encode(uint256(keccak256("linea.storage.ContractNameStorage")) - 1)) & ~bytes32(uint256(0xff))`

## Assembly

Use hex for memory offsets:

```solidity
// Correct: hex offsets
assembly {
  mstore(add(mPtr, 0x20), _var)
  mstore(add(mPtr, 0x40), _otherVar)
}

// Incorrect: decimal offsets
assembly {
  mstore(add(mPtr, 32), _var)
}
```
