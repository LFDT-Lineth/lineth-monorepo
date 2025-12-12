// SPDX-License-Identifier: GPL-3.0
pragma solidity 0.8.26;

import {TestingBase} from "./TestingBase.sol";
import {SelfDestructible} from "./SelfDestructible.sol";

contract Factory is TestingBase {
    address public lastDeployed;

    event Deployed(address addr, uint256 salt);
    event CallSuccess(address target, uint256 value, bool didSelfDestruct);
    event EmptyDeployed(string s);

    function deploy(uint256 _salt) external returns (address) {
        bytes memory bytecode = type(SelfDestructible).creationCode;

        address addr;
        assembly {
            addr := create2(0, add(bytecode, 0x20), mload(bytecode), _salt)
            if iszero(addr) { revert(0, 0) }
        }

        lastDeployed = addr;
        emit Deployed(addr, _salt);
        return addr;
    }

    function callDeployed(uint256 value, bool destroy) external {
        require(lastDeployed != address(0), "No deployed contract");

        // interface call using low-level call
        (bool success, ) = lastDeployed.call(
            abi.encodeWithSignature("modifyOrDestroy(uint256,bool)", value, destroy)
        );

        address deployed = lastDeployed;
        uint256 s;
        assembly {
            s := extcodesize(deployed)
        }

        if (s == 0) { emit EmptyDeployed("No deployed code"); }

        require(success, "Call failed");
        emit CallSuccess(lastDeployed, value, destroy);
    }

    function callMain(bool _touchStorage, bool _modifyStorage, bool _selfdestruct, bool _revert) external {
        require(lastDeployed != address(0), "No deployed contract");

        // interface call using low-level call
        (bool success, ) = lastDeployed.call(
            abi.encodeWithSignature("main(bool,bool,bool)", _touchStorage, _modifyStorage, _selfdestruct)
        );

        address deployed = lastDeployed;
        uint256 s;
        assembly {
            s := extcodesize(deployed)
        }

        if (s == 0) { emit EmptyDeployed("No deployed code"); }

        require(success, "Call failed");

        if (_revert == true){
            assembly{
                revert(0, 0)
            }
        }
    }
}
