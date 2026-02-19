# NatSpec Docstrings

**Impact: CRITICAL (prevents documentation gaps and improves discoverability)**

**ALWAYS use NatSpec docstrings for all public/external items.** This is critical for:
- Consumer documentation via interfaces
- Block explorer documentation

## Requirements

- Every public/external function MUST have `@notice`
- Every function parameter MUST have `@param _paramName` (in the same order as the function signature)
- Every return value MUST have `@return variableName`
- Events MUST document all parameters (in order)
- Errors MUST explain when they are thrown
- Use `DEPRECATED` in NatSpec docstrings for deprecated items
- Every contract/interface NatSpec block MUST include `@author Consensys Software Inc.` and `@custom:security-contact security-report@linea.build`
- Use `@dev` on any function to document non-straightforward implementations, design choices, or key restrictions that consumers or maintainers should be aware of. This applies to any function visibility.

## Contract-Level NatSpec

Every contract and interface MUST include `@author` and `@custom:security-contact`:

```solidity
/**
 * @title Native yield extension module for the Linea L1MessageService.
 * @author Consensys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
abstract contract LineaRollupYieldExtension is LineaRollupBase, ILineaRollupYieldExtension {
```

## `@dev` Tag

Use `@dev` to document non-straightforward implementations, design choices, or key restrictions. This applies to any function visibility.

```solidity
/**
 * @notice Anchors the final state root for the given block range.
 * @dev Uses the finalization data's parentStateRootHash to validate chain continuity.
 * @param _finalizationData The finalization data for the block range.
 */
function finalizeBlocks(FinalizationData calldata _finalizationData) external;
```

## Examples

### Correct: Complete NatSpec docstring

```solidity
/**
 * @notice Sends a message to L2.
 * @param _to The recipient address on L2.
 * @param _fee The fee amount in wei.
 * @param _calldata The message calldata.
 * @return messageHash The hash of the sent message.
 */
function sendMessage(
  address _to,
  uint256 _fee,
  bytes calldata _calldata
) external payable returns (bytes32 messageHash);
```

### Incorrect: Missing NatSpec docstring

```solidity
function sendMessage(
  address _to,
  uint256 _fee,
  bytes calldata _calldata
) external payable returns (bytes32);
```

### Incorrect: Parameters out of order

```solidity
/**
 * @notice Sends a message to L2.
 * @param _calldata The message calldata.
 * @param _to The recipient address on L2.
 * @param _fee The fee amount in wei.
 */
function sendMessage(
  address _to,
  uint256 _fee,
  bytes calldata _calldata
) external payable;
```

## Events

Document all event parameters in order:

```solidity
/**
 * @notice Emitted when a message is sent.
 * @param sender The address that sent the message.
 * @param to The recipient address.
 * @param messageHash The hash of the message.
 */
event MessageSent(address indexed sender, address indexed to, bytes32 messageHash);
```

## Errors

Explain when errors are thrown:

```solidity
/**
 * @notice Thrown when the provided fee is insufficient.
 * @param provided The fee amount provided.
 * @param required The minimum fee required.
 */
error InsufficientFee(uint256 provided, uint256 required);
```

## Deprecation

Use `DEPRECATED` in NatSpec docstrings for deprecated items:

```solidity
/**
 * @notice DEPRECATED: Use sendMessageV2 instead.
 * @dev This function will be removed in the next major version.
 */
function sendMessage(address _to) external;
```

For deprecated state variables, also change visibility to `private` and append `_DEPRECATED` to the name.
This prevents external consumers from depending on stale storage slots.

```solidity
// Correct: private visibility + _DEPRECATED suffix
uint256 private _someVar_DEPRECATED;

// Incorrect: keeping original visibility
uint256 public someVar;
```
