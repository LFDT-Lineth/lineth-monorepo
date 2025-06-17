// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.30;

import { IPlonkVerifier } from "../../verifiers/interfaces/IPlonkVerifier.sol";

contract IntegrationTestTrueVerifier is IPlonkVerifier {
  /// @dev Always returns true for quick turnaround testing.
  function Verify(bytes calldata, uint256[] calldata) external pure returns (bool) {
    return true;
  }
}
