// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.26;

/**
 * @title Interface declaring verifier functions.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
interface IPlonkVerifier {
  /**
   * @notice Interface for verifier contracts.
   * @param _proof The proof used to verify.
   * @param _public_inputs The computed public inputs for the proof verification.
   * @return success Returns true if successfully verified.
   */
  function Verify(bytes calldata _proof, uint256[] calldata _public_inputs) external returns (bool success);
}
