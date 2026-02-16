// SPDX-License-Identifier: AGPL-3.0
pragma solidity 0.8.33;

import { LineaRollupBase } from "../LineaRollupBase.sol";

/**
 * @title Contract to manage cross-chain messaging on L1, L2 data submission, and rollup proof verification.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
contract ExternalShnarfStorageRollup is LineaRollupBase {
  /// @custom:oz-upgrades-unsafe-allow constructor
  constructor() {
    _disableInitializers();
  }

  /**
   * @notice Initializes LineaRollup and underlying service dependencies - used for new networks only.
   * @dev DEFAULT_ADMIN_ROLE is set for the security council.
   * @dev OPERATOR_ROLE is set for operators.
   * @dev Note: This is used for new testnets and local/CI testing, and will not replace existing proxy based contracts.
   * @param _initializationData The initial data used for proof verification.
   */
  function initialize(BaseInitializationData calldata _initializationData) external initializer {
    __LineaRollup_init(
      _initializationData,
      _computeShnarf(EMPTY_HASH, EMPTY_HASH, _initializationData.initialStateRootHash, EMPTY_HASH, EMPTY_HASH)
    );
  }

  /**
   * @notice Returns if the shnarf exists.
   * @dev Value > 0 means that it exists. Default is 1.
   * @param _shnarf The shnarf being checked for existence.
   * @return shnarfExists The shnarf's existence value.
   */
  function blobShnarfExists(bytes32 _shnarf) public view returns (uint256 shnarfExists) {
    shnarfExists = shnarfProvider.blobShnarfExists(_shnarf);
  }

  /**
   * @notice Returns the ABI version and not the reinitialize version.
   * @return contractVersion The contract ABI version.
   */
  function CONTRACT_VERSION() public view virtual override returns (string memory contractVersion) {
    contractVersion = "1.0";
  }
}
