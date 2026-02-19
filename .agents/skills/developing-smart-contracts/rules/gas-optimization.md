# Gas Optimization Rules

**Impact: CRITICAL (performance and user cost)**

Gas efficiency is critical for Linea contracts. Apply these rules unless there is a
documented safety, audit, or readability reason to deviate.

## Calldata and External Functions

- Use `external` for functions that accept large arrays or structs to avoid
  copying to memory.
- Use `calldata` for read-only dynamic inputs in external functions.

```solidity
// Correct: external + calldata
function submit(bytes32[] calldata _proofs) external {
  _verify(_proofs);
}

// Incorrect: public + memory
function submit(bytes32[] memory _proofs) public {
  _verify(_proofs);
}
```

## Minimize Storage Reads and Writes

- Cache storage values used multiple times in a function.
- Avoid redundant writes (write only when value changes).

```solidity
// Correct: cache storage read
uint256 current = fee;
require(current != 0, FeeNotSet());
_charge(current);

// Incorrect: repeated storage reads
require(fee != 0, FeeNotSet());
_charge(fee);
```

## Memory Usage

- Avoid copying `calldata` to memory unless required (e.g., a struct whose fields are read many times may be cheaper to copy to memory once).
- Avoid `abi.encode`/`abi.encodePacked` in tight loops unless measured and necessary.
- Use storage pointers when updating multiple fields in a struct.

```solidity
// Correct: operate on calldata directly
function submit(bytes32[] calldata _proofs) external {
  _verify(_proofs);
}

// Incorrect: unnecessary memory copy
function submit(bytes32[] calldata _proofs) external {
  bytes32[] memory proofs = _proofs;
  _verify(proofs);
}
```

```solidity
// Correct: storage pointer for multiple updates
User storage user = users[_id];
user.balance += _amount;
user.lastUpdated = block.timestamp;
```

## Use Custom Errors

Custom errors are cheaper than revert strings and should be preferred.

```solidity
// Correct
error Unauthorized();
require(msg.sender == owner, Unauthorized());

// Incorrect
require(msg.sender == owner, "Unauthorized");
```

## Tight Loops and Unchecked Math

- Use `unchecked` only when you can prove the arithmetic cannot overflow or underflow.
- Keep the invariant in a short comment when using `unchecked`.
- Add a `// TODO: Manual review for unchecked assumption` comment to flag for audit.

```solidity
// Correct
for (uint256 i; i < items.length; ) {
  _process(items[i]);
  unchecked { ++i; } // i < items.length — TODO: Manual review for unchecked assumption
}
```

## Short-Circuit Expensive Checks

Order checks to fail early and avoid unnecessary work.

```solidity
// Correct: cheap check first
require(_to != address(0), ZeroAddressNotAllowed());
require(_isEligible(_to), NotEligible());

// Incorrect: expensive check first
require(_isEligible(_to), NotEligible());  // reads storage
require(_to != address(0), ZeroAddressNotAllowed());  // cheap comparison
```

## Avoid Unbounded Work

Gas costs scale with work done. Prefer batching with explicit limits and
documented expectations for max sizes.

```solidity
// Correct: explicit batch limit
function process(uint256[] calldata _ids) external {
  require(_ids.length <= MAX_BATCH_SIZE, BatchTooLarge());
  // ...
}

// Incorrect: no limit on batch size
function process(uint256[] calldata _ids) external {
  for (uint256 i; i < _ids.length; ) {
    // unbounded loop can exceed block gas limit
  }
}
```