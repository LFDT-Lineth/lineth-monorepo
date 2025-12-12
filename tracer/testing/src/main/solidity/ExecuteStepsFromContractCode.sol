// SPDX-License-Identifier: MIT
pragma solidity 0.8.26;

import {TestingFrameworkStorageLayout} from "./TestingFrameworkStorageLayout.sol";
import {TestingBase} from "./TestingBase.sol";

/**
 * @notice Sample contract that reads data from a contract's code as bytes, converts to external calls and executes them.
 */
contract ExecuteStepsFromContractCode is
    TestingFrameworkStorageLayout,
    TestingBase
{
    // Compile steps into ContractCall array
    // Call encodeCallsToContract with the array getting the address back for storage
    // In a function or externally you can call getContractBasedStepsAndExecute with that address
}
