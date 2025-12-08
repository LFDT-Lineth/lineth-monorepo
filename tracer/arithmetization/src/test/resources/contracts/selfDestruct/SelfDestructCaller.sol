// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

contract SelfDestructCaller {
    function callSelfDestructCalleeWhenWarm(address selfDestructCalleeAddress)
        external
    {
        // Get the balance of SelfDestructCallee to warm it
        emit Balance(address(selfDestructCalleeAddress).balance);

        // Get the codesize of SelfDestructCallee
        uint256 size;
        assembly {
            size := extcodesize(selfDestructCalleeAddress)
        }
        emit ExtCodeSize(size);

        // Perform the STATICCALL
        // (bool success, ) = selfDestructCalleeAddress.staticcall(abi.encodeWithSignature("invokeSelfDestruct()")); // fails
        // emit CallSuccess(success);

        // Perform the CALL
        (bool success, ) = selfDestructCalleeAddress.call(
            abi.encodeWithSignature("invokeSelfDestruct()")
        ); // succeed
        emit CallSuccess(success);

        // here codesize of SelfDestructCallee is not 0 yet
    }

    function verifyBalanceAndCodesizeOfSelfDestructCallee(
        address selfDestructCalleeAddress
    ) external {
        // here codesize of SelfDestructCallee is 0 after invoking callSelfDestructCalleeWhenWarm
        // Get the balance of SelfDestructCallee
        emit Balance(address(selfDestructCalleeAddress).balance);

        // Get the codesize of SelfDestructCallee
        uint256 size;
        assembly {
            size := extcodesize(selfDestructCalleeAddress)
        }
        emit ExtCodeSize(size);
    }

    event Balance(uint256 balance);

    event ExtCodeSize(uint256 size);

    event CallSuccess(bool success);
}
