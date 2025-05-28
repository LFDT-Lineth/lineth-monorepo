// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

contract ExcludedPrecompiles {
    function callRIPEMD160(bytes memory data) public view returns (bytes20 result) {
        // The RIPEMD-160 precompile is located at address 0x3
        address ripemdPrecompile = address(0x3);
        
        // Prepare the input data
        bytes memory input = data;
        
        // Prepare variables for the assembly call
        bool success;
        
        // Use inline assembly to call the precompile
        assembly {
            // Call the precompile
            // Arguments: gas, address, input offset, input size, output offset, output size
            success := staticcall(gas(), ripemdPrecompile, add(input, 32), mload(input), result, 20)
        }
        
        // Check if the call was successful
        require(success, "RIPEMD-160 call failed");
        
        return result;
    }

    function callBlake2f(
        uint32 rounds,
        bytes32[2] memory h,
        bytes32[4] memory m,
        bytes8[2] memory t,
        bool f
    ) public view returns (bytes32[2] memory) {
        // Blake2f precompile address
        address BLAKE2F_PRECOMPILE = address(0x09);

        bytes memory input = abi.encodePacked(
            rounds,
            h[0], h[1],
            m[0], m[1], m[2], m[3],
            t[0], t[1],
            f ? bytes1(0x01) : bytes1(0x00)
        );

        (bool success, bytes memory result) = BLAKE2F_PRECOMPILE.staticcall(input);
        require(success, "Blake2f precompile call failed");

        bytes32[2] memory output;
        assembly {
            mstore(output, mload(add(result, 32)))
            mstore(add(output, 32), mload(add(result, 64)))
        }

        return output;
    }
}