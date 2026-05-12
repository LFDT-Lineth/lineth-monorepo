/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
pragma solidity >=0.7.0 <0.9.0;

contract DummyAdder {
    uint sum;

    function add(int count) public {
        for (int i = 0; i < count; i++) {
            sum++;
        }
    }
}
