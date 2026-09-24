// SPDX-License-Identifier: AGPL-3.0
pragma solidity 0.8.33;
import { LinethRollupBase } from "./LinethRollupBase.sol";
import { DataRollingHashAcceptor } from "./dataAvailability/DataRollingHashAcceptor.sol";
import { LocalDataRollingHashProvider } from "./dataAvailability/LocalDataRollingHashProvider.sol";

/**
 * @title Contract to manage Validium cross-chain messaging on L1 and proof verification.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
contract Validium is LinethRollupBase, LocalDataRollingHashProvider, DataRollingHashAcceptor {
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
    __LinethRollup_init(_initializationData);
  }

  /**
   * @notice Returns the ABI version and not the reinitialize version.
   * @return contractVersion The contract ABI version.
   */
  function CONTRACT_VERSION() public view virtual override returns (string memory contractVersion) {
    contractVersion = "3.0";
  }
}
