// SPDX-License-Identifier: Apache-2.0
pragma solidity 0.8.33;

import { LidoStVaultYieldProvider } from "./LidoStVaultYieldProvider.sol";
import { ErrorUtils } from "../libraries/ErrorUtils.sol";

/**
 * @notice Deploys LidoStVaultYieldProvider contract.
 * @custom:security-contact security-report@linea.build
 */
contract LidoStVaultYieldProviderFactory {
  /**
   * @notice Emitted whenever LidoStVaultYieldProviderFactoryDeployed is deployed.
   * @param l1MessageService The Linea L1MessageService, also the withdrawal reserve holding contract.
   * @param yieldManager The Linea YieldManager.
   * @param vaultHub Lido VaultHub contract.
   * @param vaultFactory Lido VaultFactory contract.
   * @param steth Lido stETH contract.
   * @param _validatorContainerProofVerifier Linea ValidatorContainerProofVerifier contract.
   */
  event LidoStVaultYieldProviderFactoryDeployed(
    address l1MessageService,
    address yieldManager,
    address vaultHub,
    address vaultFactory,
    address steth,
    address _validatorContainerProofVerifier
  );

  /**
   * @notice Emitted whenever a new LidoStVaultYieldProvider is deployed.
   * @param providerAddress The newly created LidoStVaultYieldProvider address.
   */
  event LidoStVaultYieldProviderCreated(address indexed providerAddress);

  /// @notice The Linea L1MessageService, also the withdrawal reserve holding contract.
  address public immutable L1_MESSAGE_SERVICE;

  /// @notice The Linea YieldManager.
  address public immutable YIELD_MANAGER;

  /// @notice Lido VaultHub contract.
  address public immutable VAULT_HUB;

  /// @notice Lido VaultFactory contract.
  address public immutable VAULT_FACTORY;

  /// @notice Lido stETH contract.
  address public immutable STETH;

  /// @notice Linea ValidatorContainerProofVerifier contract.
  address public immutable VALIDATOR_CONTAINER_PROOF_VERIFIER;

  /// @notice Used to set immutable variables, but not storage.
  /// @param _l1MessageService The Linea L1MessageService, also the withdrawal reserve holding contract.
  /// @param _yieldManager The Linea YieldManager.
  /// @param _vaultHub Lido VaultHub contract.
  /// @param _vaultFactory Lido VaultFactory contract.
  /// @param _steth Lido stETH contract.
  /// @param _validatorContainerProofVerifier Linea ValidatorContainerProofVerifier contract.
  constructor(
    address _l1MessageService,
    address _yieldManager,
    address _vaultHub,
    address _vaultFactory,
    address _steth,
    address _validatorContainerProofVerifier
  ) {
    ErrorUtils.revertIfZeroAddress(_l1MessageService);
    ErrorUtils.revertIfZeroAddress(_yieldManager);
    ErrorUtils.revertIfZeroAddress(_vaultHub);
    ErrorUtils.revertIfZeroAddress(_vaultFactory);
    ErrorUtils.revertIfZeroAddress(_steth);
    ErrorUtils.revertIfZeroAddress(_validatorContainerProofVerifier);
    L1_MESSAGE_SERVICE = _l1MessageService;
    YIELD_MANAGER = _yieldManager;
    VAULT_HUB = _vaultHub;
    VAULT_FACTORY = _vaultFactory;
    STETH = _steth;
    VALIDATOR_CONTAINER_PROOF_VERIFIER = _validatorContainerProofVerifier;

    emit LidoStVaultYieldProviderFactoryDeployed(
      _l1MessageService,
      _yieldManager,
      _vaultHub,
      _vaultFactory,
      _steth,
      _validatorContainerProofVerifier
    );
  }

  /**
   * @notice Creates LidoStVaultYieldProvider instance.
   * @dev LidoStVaultYieldProvider Initialization is handled via permissioned YieldManager.addYieldProvider().
   * @return yieldProviderAddress The address of the deployed LidoStVaultYieldProvider beacon proxy.
   */
  function createLidoStVaultYieldProvider() external returns (address yieldProviderAddress) {
    yieldProviderAddress = address(
      new LidoStVaultYieldProvider(
        L1_MESSAGE_SERVICE,
        YIELD_MANAGER,
        VAULT_HUB,
        VAULT_FACTORY,
        STETH,
        VALIDATOR_CONTAINER_PROOF_VERIFIER
      )
    );
    emit LidoStVaultYieldProviderCreated(yieldProviderAddress);
  }
}
