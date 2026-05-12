// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

// example of input:
// 0x000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000001aabbcc
contract BLS12_MAP_FP_TO_G1 {
    function callBLS12_MAP_FP_TO_G1(bytes memory input)
        public
        view
        returns (bytes memory)
    {
        uint256 callDataSize = input.length;
        bytes memory output = new bytes(128); // Allocate memory for the output (BLS12_MAP_FP_TO_G1 returns 128 bytes)

        assembly {
            let callDataOffset := add(input, 0x20)  // Move pointer past length prefix to actual input
            let returnAtOffset := add(output, 0x20) // Move pointer past length prefix to store output

            let success := staticcall(
                gas(),
                0x10, // BLS12_MAP_FP_TO_G1 address
                callDataOffset,
                callDataSize,
                returnAtOffset,
                128  // returnAtCapacity (128 bytes)
            )
        }
        return output;
    }
}