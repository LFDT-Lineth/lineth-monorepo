// SPDX-License-Identifier: MIT
pragma solidity ^0.4.24;

contract FundsReceiver1 {
    function() external payable {}

    function tipTheSender(bool _useCallCode) external payable {
        uint256 refundAmount = msg.value / 2;
        bool success;
        if (_useCallCode) {
            success = msg.sender.callcode.value(refundAmount)("");
        } else {
            success = msg.sender.call.value(refundAmount)("");
        }
        require(success, "FR1 could not tip the sender");
    }
}
