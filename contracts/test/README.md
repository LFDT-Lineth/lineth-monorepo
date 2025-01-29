# Smart Contract Testing Guidelines

The following document serves to be a guide on the best practices for the Linea smart contract testing codebase.

**Note:** some areas might not conform to the practices - feel free to open a PR to have them comply. Work is constantly being done to improve.

The current supported framework is Hardhat and TypeScript testing. Future improvements will include Foundry.

## Folder Structure and Naming Conventions

In order to make it easy to navigate between the smart contracts themselves and the tests, please keep to the following:

### Folders
- Maintain a 1:1 structure with the contract directory for ease of reference.
- Folder names are camelCase unless they are the contract name or contract behavior which is TitleCase.
```
# Contracts
/contracts/src/messaging/l1/

# Tests
/contracts/test/hardhat/messaging/l1
/contracts/test/hardhat/rollup/LineaRollup/Finalization.ts
```

- All common or shared utilities and helpers that are test specific should be placed in `/contracts/test/hardhat/common/`.
- All common or shared utilities and helpers that are specific across the contracts (e.g. deployment scripts) should be placed in `/contracts/common/`.

### Test Files
- Maintain a 1:1 structure with the contract directory for ease of reference.
 Example:
    ```plaintext
    ./messaging/l1/L1MessageManager.ts
    ```

- If a contract is complex and requires multiple files to simplify, create a subfolder with individual test files representing behavior.

    Example:
    ```plaintext
    /contracts/src/LineaRollup/Finalization.ts
    /contracts/src/LineaRollup/BlobSubmission.ts
    /contracts/src/LineaRollup/CalldataDataSubmission.ts
    /contracts/src/LineaRollup/PermissionsOrOtherThings.ts
    ```

- Contract-specific or feature-specific helper or utility functions should be in a separate file within a child folder named `helpers/contractName.ts`.

    Example:
    ```plaintext
    /contracts/src/LineaRollup/Finalization.ts
    /contracts/src/LineaRollup/helpers/finalization.ts
    /contracts/src/LineaRollup/helpers/blobSubmission.ts
    ```

### Utilities
- Common functions and helper files should be defined by functionality that is not specific to any given contract.
    - Example: test assertions, hashing, and general framework behavior.
- Each helper file should contain functionality with a single reason to change (e.g. hashing, timing, encoding).
- File naming convention should describe the whole functionality using adverbial language (e.g. hashing).

## Test Layout
Within the test file itself, organize tests by contract name, external calls, behavior, starting with initialization/deployments, upgrades, failure tests followed by success tests. Following this order provides the following benefits:

1. Splitting by external call shows the exact contract entry points and makes permission checks easier to isolate.
2. It is easy to easily read the expected contract behavior.
3. Finding missing cases should be simpler.
4. Knowing where you expect reverts or expected permissions coming first helps to not miss/forget those scenarios.

### Describe
- Contract name (e.g., `Rate Limiter`).
    - External call (e.g., `When initializing`).
        - Expected behavior (if there are a few subcategories of behavior, this could be broken down further as well).
            - Failure tests first.  (e.g., `Should revert if limit is zero`).
                - Tests should follow the order of code paths (failure ordering for ease of code reading).
            - Success tests last. (e.g., `Should have values set`).

## Additional Guidelines

### Ethers Wrapping/Abstractions
- Ensure proper wrapping and abstraction using Ethers.js. Ideally Ethers should be in the common helpers/constants files and

### Solidity Test Contracts
- Use Solidity test contracts where necessary to set required state to simplify testing.

### Event Testing
- Test events, including expected arguments.
- Use the wrapped helper functions e.g. `expectEvent`

### Error Testing
- Test errors, including expected arguments.
- Use the wrapped helper functions e.g. `expectRevertWithCustomError`

### Scenario Building Pre-Testing
- Ideally build the scenarios in the test files without actual tests to ensure comprehensive coverage and ease of reading.

### Coverage
- Ensure high test coverage for all contracts. (ideally 100% - only some impossible test cases are allowed)

### Storage Layout and Upgrade Automated Testing
- Implement automated testing for storage layout and upgrades.
