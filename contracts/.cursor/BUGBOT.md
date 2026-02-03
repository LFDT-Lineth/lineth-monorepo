# BUGBOT Rules

## Core Mission

Automated code quality enforcement and documentation validation for Linea smart contracts.

## Execution Protocol

### 1. Smart Contract Guidelines

- **ALWAYS** load and reference [smart-contract-guidelines](../../.cursor/rules/smart-contract-guidelines/RULE.md)
- Applies to all `.sol` files in `contracts/` and `contracts-tge/`

### 2. NatSpec Documentation Checks

- Verify every public/external function has `@notice`
- Verify every parameter has `@param` (in the same order as function signature)
- Verify every return value has `@return`
- Verify events document all parameters in order
- Verify errors explain when they are thrown
- Check for `DEPRECATED` tags on deprecated items

### 3. Code Style Checks

- Verify correct license headers (Apache-2.0 for interfaces, AGPL-3.0 for contracts)
- Verify named imports are used (not wildcard imports)
- Check naming conventions:
  - Public state: `camelCase`
  - Private/internal state: `_camelCase`
  - Constants: `UPPER_SNAKE_CASE`
  - Function params: `_camelCase`
  - Init functions: `__ContractName_init`
- Verify constants are `internal` unless explicitly needed public
- Check for magic numbers (should use named constants)

### 4. File Structure Checks

- Verify correct ordering: Constants, State, Structs, Enums, Events, Errors, Modifiers, Functions
- Verify contract/interface has `@title`, `@author`, `@custom:security-contact`

Use the rules in [smart-contract-guidelines](../../.cursor/rules/smart-contract-guidelines/RULE.md) to enforce code quality and documentation standards.
