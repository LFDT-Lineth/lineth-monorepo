/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
pragma solidity >=0.7.0 <0.9.0;

contract RevertExample {
    uint256 public value;

    function setValue(uint256 _newValue) public {
        require(_newValue != 0, "Value cannot be zero");
        value = _newValue;
    }

    function forceRevert() public pure {
        revert("This function always reverts");
    }

    function conditionalRevert(uint256 _input) public pure returns (uint256) {
        if (_input < 10) {
            revert("Input must be 10 or greater");
        }
        return _input * 2;
    }
}
