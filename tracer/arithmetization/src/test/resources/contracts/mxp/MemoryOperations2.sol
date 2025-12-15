// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

contract MemoryOperations2 {

    function returnExec(uint256 offset) public pure { // , uint256 size) public pure {
        // Mstore some data and return them
        assembly {
            mstore(offset, 0xffffffffffffffff111111111111111122222222222222223333333333333333)
            mstore(add(offset, 32), 0xffffffffffffffff444444444444444455555555555555556666666666666666)
            mstore(add(offset, 64), 0xffffffffffffffff777777777777777788888888888888889999999999999999)
            mstore(add(offset, 96), 0xffffffffffffffffaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbcccccccccccccccc)
            return(offset, 128)
        }
    }

    function revertExec(uint256 offset) public pure { // }, uint256 size) public pure {
        // Mstore some data and revert
        assembly {
            mstore(offset, 0xffffffffffffffff111111111111111122222222222222223333333333333333)
            mstore(add(offset, 32), 0xffffffffffffffff444444444444444455555555555555556666666666666666)
            mstore(add(offset, 64), 0xffffffffffffffff777777777777777788888888888888889999999999999999)
            mstore(add(offset, 96), 0xffffffffffffffffaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbcccccccccccccccc)
            revert(offset, 128)
        }
    }
}
