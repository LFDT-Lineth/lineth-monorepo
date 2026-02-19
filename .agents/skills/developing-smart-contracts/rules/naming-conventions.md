# Naming Conventions

**Impact: HIGH (ensures consistency and readability across the codebase)**

## Summary Table

| Item                   | Convention       | Example                              |
| ---------------------- | ---------------- | ------------------------------------ |
| Public state           | camelCase        | `uint256 messageCount`               |
| Private/internal state | _camelCase       | `uint256 _internalCounter`           |
| Constants              | UPPER_SNAKE_CASE | `bytes32 DEFAULT_ADMIN_ROLE`         |
| Function params        | _camelCase       | `function send(address _to)`         |
| Return variables       | camelCase (named)| `returns (bytes32 messageHash)`      |
| Mappings               | descriptive keys | `mapping(uint256 id => bytes32 hash)`|
| Init functions         | __Contract_init  | `__PauseManager_init()`              |

## State Variables

### Public State

Use camelCase:

```solidity
uint256 public messageCount;
address public owner;
bool public isPaused;
```

### Private/Internal State

Prefix with underscore:

```solidity
uint256 private _internalCounter;
mapping(address => uint256) internal _balances;
```

## Constants

Use UPPER_SNAKE_CASE:

```solidity
bytes32 public constant DEFAULT_ADMIN_ROLE = keccak256("DEFAULT_ADMIN_ROLE");
uint256 private constant MAX_SUPPLY = 1000000;
```

## Function Parameters

Prefix with underscore:

```solidity
function transfer(address _to, uint256 _amount) external;
function setConfig(uint256 _maxLimit, bool _enabled) external;
```

## Return Variables

Use named returns with camelCase:

```solidity
function getBalance(address _account) external view returns (uint256 balance);
function getMessage(uint256 _id) external view returns (bytes32 messageHash, address sender);
```

## Mappings

Use descriptive key names:

```solidity
// Correct: descriptive keys
mapping(uint256 id => bytes32 hash) public messageHashes;
mapping(address account => uint256 balance) public balances;

// Incorrect: no key names
mapping(uint256 => bytes32) public messageHashes;
```

## Init Functions

Use double underscore prefix with Contract_init pattern. The `onlyInitializing` modifier is required on all init functions.

```solidity
function __PauseManager_init(address _pauserAddress) internal onlyInitializing;
function __AccessControl_init() internal onlyInitializing;
```
