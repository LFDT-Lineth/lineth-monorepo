# Smart Contract Style Guide

The following document serves as the expected style guide for the Linea smart contracts:

## Licenses
- All interfaces will use `// SPDX-License-Identifier: Apache-2.0` for others to potentially consume
- All contracts other than specific ones will use `// SPDX-License-Identifier: AGPL-3.0`

## Imports
All imports should be in the format of:
```
import { ImportType } from "../ImportType.sol";
```

## NatSpec
Contracts and interfaces will use the [NatSpec](https://docs.soliditylang.org/en/develop/natspec-format.html) formatting.

**Note:** Interfaces and their implementations have duplicated NatSpec because consumers might just use the interface, and block explorers might use either the interface or implementation for the documentation. The documentation should always be available.

- use `DEPRECATED` in the NatSpec for deprecated variables, errors, events and functions if they need to remain

## Visibility
- CONSTANTS should be internal unless there is an explicit reason for it to be public
- External and public functions should be limited
- If there is a function calling itself using `this.` or a function is made public to do this, reconsider the design and refactor.

## Naming
- Public state variables are `camelCase`
- Internal and private state variables are `_camelCase` starting with the `_`
- Mappings contain key and value descriptors e.g. `mapping(uint256 id=>bytes32 messageHash)`
- CONSTANTS are all in upper case separated by `_` e.g. `DEFAULT_MESSAGE_STATUS`
- Public and External function names are `camelCase` - e.g. `function hashMessage(uint256 _messageNumber) external {}`
- Internal and Private function names are `_camelCase` starting with the `_` - e.g. `function _calculateY() internal`
- All function parameters start with an `_` e.g. `function hashMessage(uint256 _messageNumber) external {}`
- Return variables should be named and are `camelCase`
- Inherited contract initialization functions should be named `__ContractName_init`. e.g. `__PauseManager_init`

## General
- Avoid magic numbers by using constants
- Name variables so that their intent is easy to understand
- In assembly memory mappings use hexidecimal values for memory offsets - e.g `mstore(add(mPtr, 0x20), _varName)`

## Linting
Be sure to run `pnpm run lint:fix` in the contracts folder or `pnpm run -F contracts lint:fix` from the repository root.

## File layout
### Interface Structure
All interfaces should be laid out in the following format from top to bottom:

```
// SPDX-License-Identifier: Apache-2.0
pragma solidity >=0.8.19 <=0.8.26; 

// imports here
import { ImportType } from "../ImportType.sol";

/**
 * @title Title explaining interface contents.
 * @author Author here.
 * @custom:security-contact security-report@linea.build
 */
interface ISampleContract {
	// All items have NatSpec
	// 1. Structs
	// 2. Enums
	// 3. Events with NatSpec (including parameters)
	// 4. Errors with NatSpec explaining when thrown
	// 5. External Functions
}
```

### Library Structure
All libraries should be laid out in the following format from top to bottom:

```
// SPDX-License-Identifier: AGPL-3.0
pragma solidity >=0.8.19 <=0.8.26; 

// imports here
import { ImportType } from "../ImportType.sol";
library SampleLibrary {
	// All items have NatSpec
	// 1. CONSTANTS (Public, internal and then private)
	// 2. Structs
	// 3. Enums
	// 4. Events with NatSpec (including parameters)
	// 5. Errors with NatSpec explaining when thrown
	// 6. Modifiers
	// 7. Functions (Public, external, internal and then private)
}
```

### Contract Structure

All contracts should be laid out in the following format from top to bottom:

```
// SPDX-License-Identifier: AGPL-3.0
pragma solidity >=0.8.19 <=0.8.26; 
contract SampleContract {
	// All items have NatSpec
	// 1. CONSTANTS (Public, internal and then private)
	// 2. Structs
	// 3. Enums
	// 4. Events with NatSpec (including parameters)
	// 5. Errors with NatSpec explaining when thrown
	// 6. Modifiers
	// 7. Functions (Public, external, internal and then private)
}
```