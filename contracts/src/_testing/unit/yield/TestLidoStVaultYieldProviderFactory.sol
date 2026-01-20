// SPDX-License-Identifier: AGPL-3.0
pragma solidity 0.8.33;

import { LidoStVaultYieldProviderFactory } from "../../../yield/LidoStVaultYieldProviderFactory.sol";
import { TestLidoStVaultYieldProvider } from "./TestLidoStVaultYieldProvider.sol";

import { GIndex } from "../../../yield/libs/vendor/lido/GIndex.sol";

/// @custom:oz-upgrades-unsafe-allow missing-initializer
contract TestLidoStVaultYieldProviderFactory is LidoStVaultYieldProviderFactory {
  constructor(
    address _l1MessageService,
    address _yieldManager,
    address _vaultHub,
    address _vaultFactory,
    address _steth,
    address _validatorContainerProofVerifier
  )
    LidoStVaultYieldProviderFactory(
      _l1MessageService,
      _yieldManager,
      _vaultHub,
      _vaultFactory,
      _steth,
      _validatorContainerProofVerifier
    )
  {}

  function createTestLidoStVaultYieldProvider() external returns (address yieldProviderAddress) {
    yieldProviderAddress = address(
      new TestLidoStVaultYieldProvider(
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
