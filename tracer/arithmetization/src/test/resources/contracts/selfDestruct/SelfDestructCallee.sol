// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

contract SelfDestructCallee {
    constructor() payable {}

    function invokeSelfDestruct() external {
        emit SelfDestruct();
        selfdestruct(payable(msg.sender));
    }

    event SelfDestruct();
}
