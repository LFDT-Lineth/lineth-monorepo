// SPDX-License-Identifier: AGPL-3.0
pragma solidity 0.8.33;

import { LinethRollupBase } from "../LinethRollupBase.sol";

/**
 * @title Contract to manage cross-chain messaging on L1, L2 data submission, and rollup proof verification.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
contract ExternalShnarfStorageRollup is LinethRollupBase {
  /// @custom:oz-upgrades-unsafe-allow constructor
  constructor() {
    _disableInitializers();
  }

  /**
   * @notice Initializes LinethRollup and underlying service dependencies - used for new networks only.
   * @dev DEFAULT_ADMIN_ROLE is set for the security council.
   * @dev OPERATOR_ROLE is set for operators.
   * @dev Note: This is used for new testnets and local/CI testing, and will not replace existing proxy based contracts.
   * @param _initializationData The initial data used for proof verification.
   */
  function initialize(BaseInitializationData calldata _initializationData) external initializer {
    __LinethRollup_init(_initializationData, _computePositionCommitment(EMPTY_HASH, 0));
  }

  /**
   * @notice Returns if the dataRollingHash has been anchored.
   * @dev Value > 0 means that it exists. Default is 1.
   * @param _dataRollingHash The dataRollingHash being checked for existence.
   * @return dataRollingHashExists The dataRollingHash's existence value.
   */
  function blobShnarfExists(bytes32 _dataRollingHash) public view returns (uint256 dataRollingHashExists) {
    dataRollingHashExists = shnarfProvider.blobShnarfExists(_dataRollingHash);
  }

  /**
   * @notice Returns the ABI version and not the reinitialize version.
   * @return contractVersion The contract ABI version.
   */
  function CONTRACT_VERSION() public view virtual override returns (string memory contractVersion) {
    contractVersion = "1.0";
  }
}
