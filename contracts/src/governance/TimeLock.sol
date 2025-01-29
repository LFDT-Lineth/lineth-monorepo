// SPDX-License-Identifier: AGPL-3.0
pragma solidity >=0.8.19 <=0.8.26;

import { TimelockController } from "@openzeppelin/contracts/governance/TimelockController.sol";

/**
 * @title TimeLock contract used to manage contract upgrades
 * @author ConsenSys Software Inc.
 * @notice This timelock contract will be the owner of all upgrades that gives users confidence and an ability to exit should they want to before an upgrade takes place
 * @custom:security-contact security-report@linea.build
 */
contract TimeLock is TimelockController {
  constructor(
    uint256 minDelay,
    address[] memory proposers,
    address[] memory executors,
    address admin
  ) TimelockController(minDelay, proposers, executors, admin) {}
}
