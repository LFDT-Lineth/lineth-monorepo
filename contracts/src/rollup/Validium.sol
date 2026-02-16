// SPDX-License-Identifier: AGPL-3.0
pragma solidity 0.8.33;
import { LineaRollupBase } from "./LineaRollupBase.sol";
import { ShnarfDataAcceptor } from "./dataAvailability/ShnarfDataAcceptor.sol";
import { LocalShnarfProvider } from "./dataAvailability/LocalShnarfProvider.sol";

/**
 * @title Contract to manage Validium cross-chain messaging on L1 and proof verification.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
contract Validium is LineaRollupBase, LocalShnarfProvider, ShnarfDataAcceptor {
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
    bytes32 genesisShnarf = _computeShnarf(
      EMPTY_HASH,
      EMPTY_HASH,
      _initializationData.initialStateRootHash,
      EMPTY_HASH,
      EMPTY_HASH
    );

    _blobShnarfExists[genesisShnarf] = SHNARF_EXISTS_DEFAULT_VALUE;

    __LineaRollup_init(_initializationData, genesisShnarf);
  }

  /**
   * @notice Returns the ABI version and not the reinitialize version.
   * @return contractVersion The contract ABI version.
   */
  function CONTRACT_VERSION() public view virtual override returns (string memory contractVersion) {
    contractVersion = "1.0";
  }
}
