// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.33;

/**
 * @title Interface declaring verifier functions.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
interface IPlonkVerifier {
  /**
   * @notice Chain configuration parameter structure.
   * @dev Used to configure the verifier contract with customizable parameter like chain ID, L2MessageService contract address.
   * @param _chainConfigurationHash The hash of the chain configuration.
   * @param _parameters The parameters of the chain configuration.
   */
  struct ChainConfigurationParameter {
    bytes32 value;
    string name;
  }

  /**
   * @notice Emmitted when the chain configuration is set during contract deployment.
   * @param chainConfigurationHash The hash of the chain configuration.
   * @param parameters The parameters of the chain configuration.
   */
  event ChainConfigurationSet(bytes32 chainConfigurationHash, ChainConfigurationParameter[] parameters);

  /**
   * @notice Emitted when the chain configuration is not provided.
   * @dev This error is thrown when the contract is not configured with a chain configuration.
   */
  error ChainConfigurationNotProvided();

  /**
   * @notice Get the chain configuration.
   */
  function getChainConfiguration() external view returns (bytes32);

  /**
   * @notice Interface for verifier contracts.
   * @param _proof The proof used to verify.
   * @param _public_inputs The computed public inputs for the proof verification.
   * @return success Returns true if successfully verified.
   */
  function Verify(bytes calldata _proof, uint256[] calldata _public_inputs) external returns (bool success);
}
