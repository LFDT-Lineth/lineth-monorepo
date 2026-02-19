# File Layout

**Impact: HIGH (ensures consistent structure across all contracts)**

## Pragma Version

Use **exact** version for concrete contracts and **caret** version for interfaces, abstract contracts, and libraries (anything expected to be inherited or composed):

- **Contracts**: `pragma solidity 0.8.33;` (exact)
- **Interfaces, abstract contracts, libraries**: `pragma solidity ^0.8.33;` (caret)

```solidity
// Correct: exact version for a concrete contract
pragma solidity 0.8.33;

// Correct: caret version for an interface
pragma solidity ^0.8.33;

// Incorrect: caret version on a concrete contract
// pragma solidity ^0.8.33; // in MyContract.sol

// Incorrect: exact version on an interface
// pragma solidity 0.8.33; // in IMyContract.sol
```

## Interface Structure

```solidity
// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.33;

import { ImportType } from "../ImportType.sol";

/**
 * @title Brief description.
 * @author Consensys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
interface ISampleContract {
  // 1. Structs
  struct MessageInfo {
    address sender;
    uint256 timestamp;
  }

  // 2. Enums
  enum Status {
    Pending,
    Completed,
    Failed
  }

  // 3. Events (with NatSpec docstrings)
  /**
   * @notice Emitted when a message is sent.
   * @param sender The message sender.
   * @param messageHash The hash of the message.
   */
  event MessageSent(address indexed sender, bytes32 messageHash);

  // 4. Errors (with NatSpec docstrings explaining when thrown)
  /**
   * @notice Thrown when the caller is not authorized.
   */
  error UnauthorizedCaller();

  // 5. External Functions
  /**
   * @notice Sends a message.
   * @param _to The recipient address.
   */
  function sendMessage(address _to) external;
}
```

## Contract Structure

```solidity
// SPDX-License-Identifier: AGPL-3.0
pragma solidity 0.8.33;

import { ISampleContract } from "./interfaces/ISampleContract.sol";
import { SafeERC20, IERC20 } from "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";

/**
 * @title Brief description.
 * @author Consensys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
contract SampleContract is ISampleContract {
  // 1. Using statements (for library extensions)
  using SafeERC20 for IERC20;

  // 2. Constants (public, internal, private)
  bytes32 public constant DEFAULT_ADMIN_ROLE = keccak256("DEFAULT_ADMIN_ROLE");
  uint256 internal constant MAX_MESSAGES = 1000;

  // 3. State variables
  uint256 public messageCount;
  mapping(uint256 id => bytes32 hash) public messageHashes;

  // 4. Structs (if not in interface)

  // 5. Enums (if not in interface)

  // 6. Events (with NatSpec docstrings) - if not in interface

  // 7. Errors (with NatSpec docstrings) - if not in interface

  // 8. Modifiers
  modifier onlyAdmin() {
    // check admin
    _;
  }

  // 9. Functions (public, external, internal, private)
  function sendMessage(address _to) external override {
    // implementation
  }

  function _validateMessage(bytes calldata _data) internal pure returns (bool) {
    // internal helper
  }
}
```

## Using for Library Extensions

The `using A for B` directive attaches library functions to a type. Place these directives immediately after the contract declaration, before any other declarations.

Only add these when the code calls library functions as methods on the type (e.g., `token.safeTransfer(...)`) rather than calling them directly (e.g., `SafeERC20.safeTransfer(token, ...)`).

### Why?

**Visibility**: Library extensions are immediately visible when reading the contract

### Correct

```solidity
contract PlonkVerifierForDataAggregation is IPlonkVerifier {
  using Mimc for *;

  // Constants come after using statements
  uint256 public constant VERSION = 1;
```

```solidity
contract TokenVault is ITokenVault {
  using SafeERC20 for IERC20;
  using Address for address;

  // State variables come after using statements
  mapping(address => uint256) public balances;
```

### Incorrect

```solidity
contract TokenVault is ITokenVault {
  uint256 public constant VERSION = 1;
  
  using SafeERC20 for IERC20;  // WRONG: using should be first
  
  mapping(address => uint256) public balances;
```

## Header Template

Every file should have this header structure:

```solidity
// SPDX-License-Identifier: [Apache-2.0 for interfaces | AGPL-3.0 for contracts]
// pragma solidity 0.8.33;   — for concrete contracts
// pragma solidity ^0.8.33;  — for interfaces, abstract contracts, libraries

// Named imports
import { Dependency } from "./path/to/Dependency.sol";

/**
 * @title Brief, one-line description.
 * @author Consensys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
```
