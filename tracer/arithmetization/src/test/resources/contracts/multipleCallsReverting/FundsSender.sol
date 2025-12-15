// SPDX-License-Identifier: MIT
pragma solidity ^0.4.24;

contract FundsSender {
    enum CallCase {
        BASE,
        SEND_ALL,
        INVOKE_TIP_THE_SENDER
    }

    bool useCallCode;

    function() external payable {}

    function turnOnUseCallCode() public {
        useCallCode = true;
    }

    function transferFunds(
        address _contractFR1,
        address _contractFR2,
        bool _mustRevert,
        CallCase _callCase
    ) external {
        require(
            address(this).balance == 21 ether,
            "FundsSender balance must be at least 21 ether"
        );

        bool success;

        success = callx(_contractFR1, 1 ether);
        require(success, "call 1 failed");

        success = callx(address(this), 2 ether);
        require(success, "call 2 failed");

        success = callx(_contractFR1, 3 ether);
        require(success, "call 3 failed");

        success = callx(_contractFR2, 4 ether);
        require(success, "call 4 failed");

        success = callx(address(this), 5 ether);
        require(success, "call 5 failed");

        if (_callCase == CallCase.BASE) {
            success = callx(_contractFR1, 6 ether);
            require(success, "call 6 failed");
            /* if useCallCode is false
            FS  ends up with 2 + 5     =  7 ether
            FR1 ends up with 1 + 3 + 6 = 10 ether
            FR2 ends uo with              4 ether
            */
        }
        if (_callCase == CallCase.SEND_ALL) {
            sendAll(_contractFR1);
            /* if useCallCode is false
            FS  ends up with 2 + 5      =  0 ether
            FR1 ends up with 1 + 3 + 13 = 17 ether
            FR2 ends uo with               4 ether
            */
        } else if (_callCase == CallCase.INVOKE_TIP_THE_SENDER) {
            invokeTipTheSender(_contractFR1, 6 ether);
            /* if useCallCode is false
            FS  ends up with 2 + 5 + 3 = 10 ether
            FR1 ends up with 1 + 3 + 3 = 7  ether
            FR2 ends uo with             4  ether
            */
        }

        require(!_mustRevert, "revert transferFunds");
    }

    function sendAll(address _contractFR1) internal {
        bool success;
        success = callx(_contractFR1, address(this).balance);
        require(success, "call 6 with send all failed");
    }

    function invokeTipTheSender(address _contractFR1, uint256 _value) internal {
        bool success;
        if (useCallCode) {
            success = _contractFR1.callcode.value(_value)(
                abi.encodeWithSignature("tipTheSender(bool)", useCallCode)
            );
        } else {
            success = _contractFR1.call.value(_value)(
                abi.encodeWithSignature("tipTheSender(bool)", useCallCode)
            );
        }
        require(success, "call 5 with invoke tip the sender failed");
    }

    // Support function
    function callx(address _contractAddress, uint256 _value)
        internal
        returns (bool)
    {
        bool success;
        if (useCallCode) {
            success = _contractAddress.callcode.value(_value)("");
        } else {
            success = _contractAddress.call.value(_value)("");
        }
        return success;
    }
}
