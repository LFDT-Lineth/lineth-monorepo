/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
pragma solidity >=0.7.0 <0.9.0;

contract SimpleStorage {
    string data;

    function set(string memory value) public {
        require(bytes(value).length != 0);
        data = value;
    }

    function get() public view returns (string memory) {
        return data;
    }
}
