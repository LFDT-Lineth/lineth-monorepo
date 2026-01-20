// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.33;

/**
 * @title ZkEvm rollup interface for pre-existing functions, events and errors.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
interface IZkEvmV2 {
  /**
   * @dev Thrown when the starting rootHash does not match the existing state.
   */
  error StartingRootHashDoesNotMatch();

  /**
   * @dev Thrown when zk proof is empty bytes.
   */
  error ProofIsEmpty();

  /**
   * @dev Thrown when zk proof type is invalid.
   */
  error InvalidProofType();

  /**
   * @dev Thrown when zk proof is invalid.
   */
  error InvalidProof();

  /**
   * @dev Thrown when the call to the verifier runs out of gas or reverts internally.
   */
  error InvalidProofOrProofVerificationRanOutOfGas(string errorReason);
}
