// SPDX-License-Identifier: AGPL-3.0
pragma solidity 0.8.33;

import { LineaRollupBase } from "./LineaRollupBase.sol";
import { Eip4844BlobAcceptor } from "./dataAvailability/Eip4844BlobAcceptor.sol";
import { IProvideShnarf } from "./dataAvailability/interfaces/IProvideShnarf.sol";
import { ClaimMessageV1 } from "../messaging/l1/v1/ClaimMessageV1.sol";
import { AccessControlUpgradeable } from "@openzeppelin/contracts-upgradeable/access/AccessControlUpgradeable.sol";
import { LivenessRecovery } from "./LivenessRecovery.sol";
import { IPermissionsManager } from "../security/access/interfaces/IPermissionsManager.sol";
import { IPauseManager } from "../security/pausing/interfaces/IPauseManager.sol";
import { LineaRollupYieldExtension } from "./LineaRollupYieldExtension.sol";
/**
 * @title Contract to manage cross-chain messaging on L1, L2 data submission, and rollup proof verification.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
contract LineaRollup is
  LineaRollupBase,
  LineaRollupYieldExtension,
  LivenessRecovery,
  Eip4844BlobAcceptor,
  ClaimMessageV1
{
  /// @custom:oz-upgrades-unsafe-allow constructor
  constructor() {
    _disableInitializers();
  }

  /**
   * @notice Initializes LineaRollup and underlying service dependencies - used for new networks only.
   * @dev DEFAULT_ADMIN_ROLE is set for the security council.
   * @dev OPERATOR_ROLE is set for operators.
   * @dev Note: This is used for new testnets and local/CI testing, and will not replace existing proxy based contracts.
   * @param _initializationData The initial data used for contract initialization.
   * @param _livenessRecoveryOperator The liveness recovery operator address.
   * @param _yieldManager The yield manager address.
   */
  function initialize(
    BaseInitializationData calldata _initializationData,
    address _livenessRecoveryOperator,
    address _yieldManager
  ) external initializer {
    bytes32 genesisShnarf = _computeShnarf(
      EMPTY_HASH,
      EMPTY_HASH,
      _initializationData.initialStateRootHash,
      EMPTY_HASH,
      EMPTY_HASH
    );

    _blobShnarfExists[genesisShnarf] = SHNARF_EXISTS_DEFAULT_VALUE;

    __LineaRollup_init(_initializationData, genesisShnarf);
    __LivenessRecovery_init(_livenessRecoveryOperator);
    __LineaRollupYieldExtension_init(_yieldManager);
  }

  /**
   * @notice Reinitializes LineaRollup and sets the _shnarfProvider to itself.
   */
  function reinitializeV8(
    IPermissionsManager.RoleAddress[] calldata _roleAddresses,
    IPauseManager.PauseTypeRole[] calldata _pauseTypeRoles,
    IPauseManager.PauseTypeRole[] calldata _unpauseTypeRoles
  ) external reinitializer(8) {
    address proxyAdmin;
    assembly {
      proxyAdmin := sload(PROXY_ADMIN_SLOT)
    }
    require(msg.sender == proxyAdmin, CallerNotProxyAdmin());

    __PauseManager_init(_pauseTypeRoles, _unpauseTypeRoles);
    __Permissions_init(_roleAddresses);

    shnarfProvider = IProvideShnarf(address(this));
  }

  /**
   * @notice Revokes `role` from the calling account.
   * @dev Liveness recovery operator cannot renounce role. Reverts with OnlyNonLivenessRecoveryOperator.
   * @param _role The role to renounce.
   * @param _account The account to renounce - can only be the _msgSender().
   */
  function renounceRole(
    bytes32 _role,
    address _account
  ) public virtual override(AccessControlUpgradeable, LivenessRecovery) {
    super.renounceRole(_role, _account);
  }
}
