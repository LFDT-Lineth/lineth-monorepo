/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
pragma solidity >=0.7.0 <0.9.0;

contract AddressCaller {
    function callAddress(address target) public {
        (bool success, ) = target.call("");
        // We don't care about success/failure, just that the CALL happens
        success;
    }

    function delegateCallAddress(address target) public {
        (bool success, ) = target.delegatecall("");
        success;
    }

    function staticCallAddress(address target) public {
        (bool success, ) = target.staticcall("");
        success;
    }

    function callCodeAddress(address target) public {
        assembly {
            let result := callcode(gas(), target, 0, 0, 0, 0, 0)
        }
    }
}