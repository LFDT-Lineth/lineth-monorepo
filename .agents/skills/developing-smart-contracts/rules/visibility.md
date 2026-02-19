# Visibility Guidelines

**Impact: MEDIUM (reduces attack surface and improves encapsulation)**

## Constants

Use `internal` unless explicitly needed public:

```solidity
// Correct: internal by default
bytes32 internal constant MESSAGE_HASH_SLOT = keccak256("message.hash.slot");

// Only if needed externally
bytes32 public constant DEFAULT_ADMIN_ROLE = keccak256("DEFAULT_ADMIN_ROLE");
```

## Functions

Minimize `external`/`public` surface area. If a function needs to be overridable, use `public virtual` - not `external virtual`.

**Why `public virtual` instead of `external virtual`?** A common override pattern is to add a modifier or a check and then call `super.functionName()`. This requires the function to be `public` because `external` functions cannot be called internally via `super`. Prefer `external` for functions that are not called internally and don't need to be overridable. For `virtual` functions, use `public` so that child contracts can extend behavior through `super`.

```solidity
// Correct: internal helper functions
function _validateInput(bytes calldata _data) internal pure returns (bool);
function _computeHash(address _sender, uint256 _nonce) internal view returns (bytes32);

// Correct: overridable function uses public virtual
function sendMessage(address _to) public virtual;

// Correct: non-overridable function uses external
function sendMessage(address _to) external;
```

## Avoid this.functionCall()

Refactor to use internal calls instead:

```solidity
// Incorrect: external self-call
function process() public virtual {
  this.validate();  // Wastes gas, breaks internal invariants
}

// Correct: internal call
function process() public virtual {
  _validate();
}

function _validate() internal {
  // validation logic
}
```

## Why Minimize Public Surface?

1. **Security**: Fewer entry points for attackers
2. **Gas efficiency**: Internal calls are cheaper
3. **Upgradability**: Easier to change internal implementation
4. **Clarity**: Clear separation of public API vs internals
