// SPDX-License-Identifier: MIT
pragma solidity 0.8.26;


import {CustomCreate2} from "./CustomCreate2.sol";

interface ICustomCreate2 {
    function create2WithInitCodeC_withValueAndRevert() external;
    function create2WithInitCodeC_noValueNoRevert() external;
}

/**
 * @notice ContractC
 */
contract ContractC {


    event ImmediateRedeploymentFail();
    event StoreInMap(uint key);
    event SelfDestruct();

    constructor() payable {
        uint256 value = msg.value;
        // deployment is done by CustomCreate2
        address from = msg.sender;
        if (value == 1) {
            storageMap[value]=from;
        } else if (value == 2) {
            try ICustomCreate2(from).create2WithInitCodeC_withValueAndRevert() {
            } catch Error(string memory _err) {
                assembly {
                    stop()
                }
            } catch (bytes memory _err) {
                emit ImmediateRedeploymentFail();
                // If no stop here, the deployed bytecode is 0x..33
                assembly {
                    stop()
                }
            }
        } else if (value == 3) {
            selfDestructOnDemand();
        } else if (value == 4) {
            revertOnDemand();
        }
    }

    mapping(uint => address) public storageMap;

    function storeInMap(uint key, address add) public {
        storageMap[key] = add;
        emit StoreInMap(key);
    }

    function callBackCustomCreate2(address addCustomCreate2) public {
        ICustomCreate2(addCustomCreate2).create2WithInitCodeC_noValueNoRevert();
    }

    function revertOnDemand() public {
        revert();
    }

    function selfDestructOnDemand() public {
        address payable thisAddr = payable(address(this));
        emit SelfDestruct();
        selfdestruct(thisAddr);
    }

    // Future usage

    /* function getDeployedCode() view public returns (bytes memory) {
        return address(this).code;
    } */

}
