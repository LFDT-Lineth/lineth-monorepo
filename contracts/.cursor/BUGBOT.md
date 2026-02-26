# Bugbot Review Instructions — contracts

> Extends [root BUGBOT.md](../../.cursor/BUGBOT.md). Smart contract-specific checks below.

## Core Mission

Automated code quality enforcement, documentation validation, and security review for Linea smart contracts. **ALWAYS** load and reference [smart-contract-guidelines](../../.cursor/rules/smart-contract-guidelines/RULE.md).

## Scope

All files in `contracts/src/**/*.sol` and `contracts/test/**`. Skip `contracts/lib/`, `typechain-types/`, and generated files.

## NatSpec Documentation Checks

- Verify every public/external function has `@notice`
- Verify every parameter has `@param` (in the same order as function signature)
- Verify every return value has `@return`
- Verify events document all parameters in order
- Verify errors explain when they are thrown
- Check for `DEPRECATED` tags on deprecated items
- Verify contract/interface has `@title`, `@author Consensys Software Inc.`, `@custom:security-contact security-report@linea.build`

## Code Style Checks

- Verify correct license headers (Apache-2.0 for interfaces, AGPL-3.0 for contracts)
- Verify named imports are used (not wildcard imports)
- Verify blank line after import block
- Check naming conventions:
  - Public state: `camelCase`
  - Private/internal state: `_camelCase`
  - Constants: `UPPER_SNAKE_CASE`
  - Function params: `_camelCase`
  - Return variables: `camelCase` (named)
  - Mappings: descriptive keys (`mapping(uint256 id => bytes32 hash)`)
  - Init functions: `__ContractName_init`
- Verify constants are `internal` unless explicitly needed public
- Check for magic numbers (should use named constants)
- Verify correct file ordering: Using statements -> Constants -> State -> Structs -> Enums -> Events -> Errors -> Modifiers -> Functions
- Pragma: exact `0.8.33` for contracts, caret `^0.8.33` for interfaces/abstract/libraries

## Security Checks

### Reentrancy

- Functions making external calls use `nonReentrant` or checks-effects-interactions pattern
- State updates happen before external calls
- `public virtual` functions callable internally have reentrancy guards

### Access Control

- State-changing functions have appropriate access modifiers
- `onlyInitializing` modifier on all init functions
- Security council address set for privileged operations

### Storage Layout (Upgradeable Contracts)

- New state variables added at the end of the storage struct (ERC-7201)
- No removed or reordered state variables
- `reinitializer(N)` version incremented for new initialization logic
- Both `initialize` and `reinitializeVN` use `reinitializer(N)` (never `initializer`)

### Gas Optimization

- `calldata` for read-only dynamic inputs in `external` functions
- Storage values cached when read multiple times
- Batch operations have explicit size limits
- Cheap checks ordered before expensive ones
- Custom errors preferred over revert strings

### Input Validation

- Zero-address checks via `ErrorUtils.revertIfZeroAddress()`
- Zero-hash checks via `ErrorUtils.revertIfZeroHash()`
- Array length validations before iteration
- Overflow-safe arithmetic or documented `unchecked` blocks

### Versioning

- Modified public interfaces create new versioned methods (V1, V2)
- Existing ABI files not overwritten; new versioned files created
- Deprecated items marked with `DEPRECATED` in NatSpec
- Deprecated state variables: `private` visibility + `_DEPRECATED` suffix

## Review Style

- Reference rule files from `.agents/skills/developing-smart-contracts/rules/` and `.cursor/rules/smart-contract-guidelines/RULE.md`
- Classify severity: **blocking**, **important**, **minor**
- Propose concrete fixes with code snippets
