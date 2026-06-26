---
name: developing-smart-contracts
description: Solidity smart contract development guidelines for Linea blockchain. Use when writing, reviewing, or refactoring Solidity contracts, or when the user asks about Solidity best practices, contract structure, or NatSpec docstrings. Covers NatSpec documentation, naming conventions, file layout, and code style.
license: AGPL-3.0
metadata:
  author: linea
  version: '1.0.0'
---

# Linea Smart Contract Development

Best practices for developing Solidity smart contracts on Linea blockchain. Contains
rules covering documentation, naming, structure, and code style.

Reference: [Linea Contract Style Guide](../../../contracts/docs/contract-style-guide.md)

## When to Apply

Reference these guidelines when:

- Writing new Solidity contracts for Linea
- Reviewing smart contract code
- Refactoring existing contracts
- Adding NatSpec docstring documentation
- Setting up contract file structure

## Licenses

// SPDX-License-Identifier: Apache-2.0 OR MIT

## Solidity Pragma

- Contracts: `0.8.33` (exact)
- Interfaces, abstract contracts, libraries: `^0.8.33` (caret)

## Rule Categories by Priority

| Priority | Category       | Impact   | Rule File                |
| -------- | -------------- | -------- | ------------------------ |
| 1        | Gas optimization    | CRITICAL | `rules/gas-optimization.md`   |
| 2        | NatSpec Docstrings | HIGH     | `rules/natspec.md`       |
| 3        | File Layout    | HIGH     | `rules/file-layout.md`   |
| 4        | Naming         | HIGH     | `rules/naming-conventions.md` |
| 5        | Imports        | MEDIUM   | `rules/imports.md`       |
| 6        | Visibility     | MEDIUM   | `rules/visibility.md`    |
| 7        | General Rules  | MEDIUM      | `rules/general-rules.md` |

Read individual rule files for detailed explanations and code examples (correct and incorrect).

## Quick Reference

### 1. Gas Optimization (CRITICAL)

**Gas efficiency is critical for Linea contracts.** Apply these rules unless there is a documented safety, audit, or readability reason to deviate.

```solidity
// Correct: external + calldata, cache storage reads
function submit(bytes32[] calldata _proofs) external {
  uint256 current = fee;
  require(current != 0, FeeNotSet());
  _verify(_proofs, current);
}

// Incorrect: public + memory, repeated storage reads
function submit(bytes32[] memory _proofs) public {
  require(fee != 0, FeeNotSet());
  _verify(_proofs, fee);
}
```

See [rules/gas-optimization.md](rules/gas-optimization.md) for details.

### 2. NatSpec Docstrings (HIGH)

**ALWAYS use NatSpec docstrings for all public/external items.**

```solidity
// Correct: complete NatSpec with @notice, @param (in signature order), @return
/**
 * @notice Sends a message to L2.
 * @param _to The recipient address on L2.
 * @param _fee The fee amount in wei.
 * @return messageHash The hash of the sent message.
 */
function sendMessage(address _to, uint256 _fee) external payable returns (bytes32 messageHash);
```

Every contract/interface NatSpec block must include `@author Consensys Software Inc.` and `@custom:security-contact security-report@linea.build`.

Consult [rules/natspec.md](rules/natspec.md) for NatSpec docstring rules and examples

### 3. File Layout (HIGH)

**Interface structure:**
1. Structs → 2. Enums → 3. Events → 4. Errors → 5. External Functions

**Contract structure:**
1. Using statements → 2. Constants → 3. State variables → 4. Structs → 5. Enums → 6. Events → 7. Errors → 8. Modifiers → 9. Functions

See [rules/file-layout.md](rules/file-layout.md) for templates.

### 4. Naming Conventions (HIGH)

See [rules/naming-conventions.md](rules/naming-conventions.md) for symbol naming conventions.

### 5. Imports (MEDIUM)

Always use named imports. Always insert a blank line after imports.

```solidity
// Correct: named imports with blank line before NatSpec/contract declaration
import { IMessageService } from "../interfaces/IMessageService.sol";
import { Ownable } from "@openzeppelin/contracts/access/Ownable.sol";

/**
 * @title Brief description.
 * @author Consensys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
contract MessageService is IMessageService, Ownable {
```

See [rules/imports.md](rules/imports.md) for details.

### 6. Visibility (MEDIUM)

See [rules/visibility.md](rules/visibility.md) for rules on applying visibility modifiers (`internal`, `external`, `public`, etc).

### 7. General Rules (MEDIUM)

See [rules/general-rules.md](rules/general-rules.md) for inheritance and general style rules

## Development Workflow

Follow this sequence when writing or modifying Solidity contracts:

1. **Write/modify contract** applying rules from the priority table above.
2. **Compile** and fix errors:
   ```bash
   pnpm -F contracts run build
   ```
3. **Run linter** and fix warnings:
   ```bash
   pnpm -F contracts run lint:fix
   ```
4. **Run tests** to verify behavior:
   ```bash
   pnpm -F contracts run test
   ```
5. **Verify NatSpec coverage**: confirm all public/external functions, events, and errors have complete docstrings with `@notice`, `@param` (in signature order), and `@return`.
6. **Review gas patterns**: check for unnecessary `memory` copies, repeated storage reads, and missing batch limits.
7. **Commit** after passing the checklist below.

## Commit Checklist

Before making a commit, please verify:

- [ ] Rules on licenses and Solidity pragma have been applied
- [ ] All public items have NatSpec docstrings (`@notice`, `@param`, `@return`)
- [ ] All rules in `rules/*.md` have been applied
- [ ] Linting passes (`pnpm -F contracts run lint:fix`)

## Commands

For commands for testing and linting, refer to [rules/commands.md](rules/commands.md)
