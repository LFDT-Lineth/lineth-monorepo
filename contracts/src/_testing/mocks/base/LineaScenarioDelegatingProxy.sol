// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.33;

import { LineaScenarioTesting } from "./LineaScenarioTesting.sol";

/// @dev Test ScenarioDelegatingProxy.
contract LineaScenarioDelegatingProxy {
  mapping(address => mapping(LineaScenarioTesting.Scenario => bool)) public executedScenarios;

  function executeScenario(LineaScenarioTesting.Scenario _scenario, uint256 _loopIterations) external returns (bool) {
    // Deploy new scenario contract to consume gas and delegate to.
    LineaScenarioTesting lineaScenarioTesting = new LineaScenarioTesting();

    // Delegate the call noting that only 63/64 of the gas will be sent into the scenario in order to handle the revert
    (bool callSuccess, ) = address(lineaScenarioTesting).delegatecall(
      abi.encodeCall(LineaScenarioTesting.executeScenario, (_scenario, _loopIterations))
    );

    // If you are testing SSTORE out of gas here post delegatecall out of gas, this will never save.
    executedScenarios[address(lineaScenarioTesting)][_scenario] = callSuccess;

    return callSuccess;
  }
}
