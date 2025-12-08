// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

contract ContractModifyingStorageInConstructor {
    // Storage variables
    uint256 public s0;
    uint256 public s1;
    uint256 public s2;
    uint256 public s3;

    // Constructor that modifies the storage variables
    constructor() {
        // Set initial values
        s0 = 0;
        s1 = 1;
        s2 = 2;
        s3 = 3;
        // Change values
        s0 = s0 + 10;
        s1 = s1 + 11;
        s2 = s2 + 12;
        s3 = s3 + 13;
        // Change values again and set s0 back to 0
        s0 = s0 * 0; 
        s1 = s1 * 20;
        s2 = s2 * 21;
        s3 = s3 * 21;
    }
}