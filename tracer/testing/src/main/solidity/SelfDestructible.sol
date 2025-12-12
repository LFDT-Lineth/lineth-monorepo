// SPDX-License-Identifier: GPL-3.0
pragma solidity 0.8.26;

import {TestingBase} from "./TestingBase.sol";

contract SelfDestructible is TestingBase {
    uint256 public storage1;
    uint256 public storage2;

    event Document(string s, bool b);

    function touchStorage(bool _do) internal {

        emit Document("Trigger SLOAD", _do);

        if (_do) {
            assembly {
                let slot := storage1.slot
                let val := sload(slot)
            }
        }
    }

    function modifyStorage(bool _do) internal {

        emit Document("Trigger SSTORE", _do);

        if (_do) {
            storage2 += 1;
        }
    }

    function maybeSelfdestruct(bool _do) internal {

        emit Document("Trigger SELFDESTRUCT", _do);

        if (_do) {
            selfdestruct(payable(msg.sender));
        }
    }

    function main(bool _touchStorage, bool _modifyStorage, bool _selfdestruct) external {
        touchStorage(_touchStorage);
        modifyStorage(_modifyStorage);
        maybeSelfdestruct(_selfdestruct);

        assembly{
            return(0, 0)
        }
    }
}