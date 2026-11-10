// SPDX-License-Identifier: AGPL-3.0
pragma solidity 0.8.33;
import { LinethRollupBase } from "./LinethRollupBase.sol";
import { DataRollingHashAcceptor } from "./dataAvailability/DataRollingHashAcceptor.sol";
import { LocalDataRollingHashProvider } from "./dataAvailability/LocalDataRollingHashProvider.sol";
import { InitializationVersionCheck } from "../common/InitializationVersionCheck.sol";

/**
 * @title Contract to manage Validium cross-chain messaging on L1 and proof verification.
 * @author Consensys Incorporated
 * @custom:security-contact security-report@linea.build
 */
contract Validium is
  InitializationVersionCheck,
  LinethRollupBase,
  LocalDataRollingHashProvider,
  DataRollingHashAcceptor
{
  /// @notice This is the ABI version and not the reinitialize version.
  string private constant _VALIDIUM_CONTRACT_VERSION = "3.0";

  /// @custom:oz-upgrades-unsafe-allow constructor
  constructor() {
    _disableInitializers();
  }

  /**
   * @notice Initializes Validium and underlying service dependencies - used for new networks only.
   * @dev DEFAULT_ADMIN_ROLE is set for the security council.
   * @dev OPERATOR_ROLE is set for operators.
   * @dev Note: This is used for new testnets and local/CI testing, and will not replace existing proxy based contracts.
   * @dev Only callable on a fresh proxy (initialized version 0). It consumes the latest reinitializer version (3) so
   *   `reinitializeValidiumV3` can never run on a freshly initialized proxy.
   * @param _initializationData The initial data used for proof verification.
   */
  function initialize(
    BaseInitializationData calldata _initializationData
  ) external onlyInitializedVersion(0) reinitializer(3) {
    __LinethRollup_init(_initializationData);
  }

  /**
   * @notice Reinitializer for v3: migrates the legacy shnarf into the blob-spanning dataRollingHash model.
   * @dev For in-place upgrades of already deployed Validium (2.0) proxies, which were initialized with `initializer`
   *   (version 1). Should be called using an upgradeAndCall transaction to the ProxyAdmin.
   * @dev `reinitializer(3)` guarantees this runs exactly once per proxy, and a proxy created with `initialize`
   *   (which also consumes version 3) can never call it.
   * @dev See `_migrateLegacyShnarf` for the migration details.
   */
  function reinitializeValidiumV3() external reinitializer(3) {
    _migrateLegacyShnarf();

    emit LineaRollupVersionChanged(bytes8("2.0"), bytes8(bytes(_VALIDIUM_CONTRACT_VERSION)));
  }

  /**
   * @notice Returns the ABI version and not the reinitialize version.
   * @return contractVersion The contract ABI version.
   */
  function CONTRACT_VERSION() public view virtual override returns (string memory contractVersion) {
    contractVersion = _VALIDIUM_CONTRACT_VERSION;
  }
}
