// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.26;

import { IPlonkVerifier } from "../../../verifiers/interfaces/IPlonkVerifier.sol";

/// @dev Test verifier contract that returns true.
contract RevertingVerifier is IPlonkVerifier {
  enum Scenario {
    EMPTY_REVERT,
    GAS_GUZZLE
  }

  Scenario private scenario;

  constructor(Scenario _scenario) {
    scenario = _scenario;
  }

  function Verify(bytes calldata, uint256[] calldata) external returns (bool) {
    _executeScenario(scenario, type(uint256).max);

    // defaults to EMPTY_REVERT scenario
    revert();
  }

  function ExecuteScenario(Scenario _scenario, uint256 _loopIterations) external returns (bool) {
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
      scenario = scenario;
    }

    return true;
  }
}
