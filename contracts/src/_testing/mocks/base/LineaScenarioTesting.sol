// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.19;

/// @dev Test scenarios on Linea.
contract LineaScenarioTesting {
  enum Scenario {
    RETURN_TRUE,
    GAS_GUZZLE
  }

  Scenario private scenario;

  function executeScenario(Scenario _scenario, uint256 _loopIterations) external returns (bool) {
    return _executeScenario(_scenario, _loopIterations);
  }

  function _executeScenario(Scenario _scenario, uint256 _loopIterations) internal returns (bool) {
    if (_scenario == Scenario.GAS_GUZZLE) {
      // guzzle the gas
      uint256 counter;
      while (counter < _loopIterations) {
        counter++;
      }

      // silencing the warning - this needs to be external to consume gas.
      scenario = _scenario;
    }

    return true;
  }
}
