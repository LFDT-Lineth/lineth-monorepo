// SPDX-License-Identifier: AGPL-3.0
pragma solidity 0.8.33;

import { LinethRollupBase } from "./LinethRollupBase.sol";
import { Eip4844BlobAcceptor } from "./dataAvailability/Eip4844BlobAcceptor.sol";
import { ClaimMessageV1 } from "../messaging/l1/v1/ClaimMessageV1.sol";
import { AccessControlUpgradeable } from "@openzeppelin/contracts-upgradeable/access/AccessControlUpgradeable.sol";
import { LivenessRecovery } from "./LivenessRecovery.sol";
import { LinethRollupYieldExtension } from "./LinethRollupYieldExtension.sol";
import { InitializationVersionCheck } from "../common/InitializationVersionCheck.sol";

/**
 * @title Contract to manage cross-chain messaging on L1, L2 data submission, and rollup proof verification.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
contract LinethRollup is
  InitializationVersionCheck,
  LinethRollupBase,
  LinethRollupYieldExtension,
  LivenessRecovery,
  Eip4844BlobAcceptor,
  ClaimMessageV1
{
  /// @custom:oz-upgrades-unsafe-allow constructor
  constructor() {
    _disableInitializers();
  }

  /**
   * @notice Initializes LinethRollup and underlying service dependencies - used for new networks only.
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
  ) external onlyInitializedVersion(0) reinitializer(10) {
    __LinethRollup_init(_initializationData);
    __LivenessRecovery_init(_livenessRecoveryOperator);
    __LinethRollupYieldExtension_init(_yieldManager);
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

  /**
   * @notice Version-bump reinitializer for v10.
   * @dev Should be called using an upgradeAndCall transaction to the ProxyAdmin for live-chain
   *   (in-place) upgrades.
   * @dev No data migration is performed here: the legacy-shnarf migration is validated and applied
   *   on-chain inside `finalizeBlocks` itself. This function only advances `CONTRACT_VERSION()`.
   */
  function reinitializeLineaRollupV10() external reinitializer(10) {
    emit LineaRollupVersionChanged(bytes8("9.0"), bytes8("10.0"));
  }
}
