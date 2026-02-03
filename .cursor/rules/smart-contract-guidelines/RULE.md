---
description: Linea Smart Contract Development Guidelines
globs: contracts/**/*.sol, contracts-tge/**/*.sol
alwaysApply: false
---

Reference: [Linea Contract Style Guide](contracts/docs/contract-style-guide.md)

# Linea Smart Contracts - Development Guidelines

## Licenses

- **Interfaces**: `// SPDX-License-Identifier: Apache-2.0`
- **Contracts**: `// SPDX-License-Identifier: AGPL-3.0`

## NatSpec Documentation

**ALWAYS use NatSpec for all public/external items.** This is critical for:
- Consumer documentation via interfaces
- Block explorer documentation

### Requirements

- Every public/external function MUST have `@notice`
- Every function parameter MUST have `@param _paramName` (in the same order as the function signature)
- Every return value MUST have `@return variableName`
- Events MUST document all parameters (in order)
- Errors MUST explain when they are thrown
- Use `DEPRECATED` in NatSpec for deprecated items

### Example

```solidity
// CORRECT
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

// WRONG - Missing NatSpec
function sendMessage(
  address _to,
  uint256 _fee,
  bytes calldata _calldata
) external payable returns (bytes32);

// WRONG - Parameters out of order
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

## Imports

Always use named imports:

```solidity
// CORRECT
import { IMessageService } from "../interfaces/IMessageService.sol";

// WRONG
import "../interfaces/IMessageService.sol";
```

## Naming Conventions

| Item | Convention | Example |
|------|------------|---------|
| Public state | camelCase | `uint256 messageCount` |
| Private/internal state | _camelCase | `uint256 _internalCounter` |
| Constants | UPPER_SNAKE_CASE | `bytes32 DEFAULT_ADMIN_ROLE` |
| Function params | _camelCase | `function send(address _to)` |
| Return variables | camelCase (named) | `returns (bytes32 messageHash)` |
| Mappings | descriptive keys | `mapping(uint256 id => bytes32 hash)` |
| Init functions | __Contract_init | `__PauseManager_init()` |

## Visibility

- **Constants**: `internal` unless explicitly needed public
- **Functions**: Minimize `external`/`public` surface area
- **Avoid**: `this.functionCall()` pattern - refactor instead

## File Layout

### Interface Structure

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
  // 2. Enums
  // 3. Events (with NatSpec)
  // 4. Errors (with NatSpec explaining when thrown)
  // 5. External Functions
}
```

### Contract Structure

```solidity
// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.33;

import { ISampleContract } from "./interfaces/ISampleContract.sol";

/**
 * @title Brief description.
 * @author Consensys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
contract SampleContract is ISampleContract {
  // 1. Constants (public, internal, private)
  // 2. State variables
  // 3. Structs
  // 4. Enums
  // 5. Events (with NatSpec)
  // 6. Errors (with NatSpec)
  // 7. Modifiers
  // 8. Functions (public, external, internal, private)
}
```

## Inheritance & Customization

When extending Linea contracts:

- Use `virtual`/`override` keywords
- Override `CONTRACT_VERSION()` for custom versions
- See examples in `src/_testing/unit/` for patterns
- **Note**: Any modifications from audited code should be independently audited

## General Rules

- **Avoid magic numbers**: Use named constants
- **Assembly**: Use hex for memory offsets (`mstore(add(mPtr, 0x20), _var)`)
- **Linting**: Run `pnpm run lint:fix` before committing

## Deployment

- Set `VERIFY_CONTRACT=true` for block explorer verification
- Use network-specific private keys (e.g., `SEPOLIA_PRIVATE_KEY`)
- See `contracts/docs/deployment.md` for full parameter reference

## Testing

- Run Solidity linting: `pnpm -F contracts run lint:sol`
- Run TypeScript linting: `pnpm -F contracts run lint:ts`
- Run tests: `pnpm -F contracts run coverage`

## Checklist

Before submitting a PR:

- [ ] All public items have NatSpec (`@notice`, `@param`, `@return`)
- [ ] Named imports used
- [ ] Naming conventions followed
- [ ] Constants are `internal` unless needed public
- [ ] Linting passes (`pnpm run lint:fix`)
- [ ] No magic numbers
