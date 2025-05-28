/*
 * Copyright Consensys Software Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with
 * the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on
 * an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations under the License.
 *
 * SPDX-License-Identifier: Apache-2.0
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