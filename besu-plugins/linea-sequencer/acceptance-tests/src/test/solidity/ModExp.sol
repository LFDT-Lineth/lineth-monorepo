// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

// example of input:
// 0x000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000001aabbcc
contract ModExp {
    function callModExp(bytes memory input)
        public
        view
        returns (bytes memory)
    {
        uint256 callDataSize = input.length;
        bytes memory output = new bytes(32); // Allocate memory for the output

        assembly {
            let callDataOffset := add(input, 0x20)  // Move pointer past length prefix to actual input
            let returnAtOffset := add(output, 0x20) // Move pointer past length prefix to store output

            let success := staticcall(
                gas(),
                0x05, // MODEXP address
                callDataOffset,
                callDataSize,
                returnAtOffset,
                0  // returnAtCapacity
            )
        }
        return output;
    }
}