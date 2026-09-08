// SPDX-License-Identifier: AGPL-3.0
pragma solidity 0.8.33;

import { LinethRollupBase } from "./LinethRollupBase.sol";
import { Eip4844BlobAcceptor } from "./dataAvailability/Eip4844BlobAcceptor.sol";
import { ClaimMessageV1 } from "../messaging/l1/v1/ClaimMessageV1.sol";
import { AccessControlUpgradeable } from "@openzeppelin/contracts-upgradeable/access/AccessControlUpgradeable.sol";
import { LivenessRecovery } from "./LivenessRecovery.sol";
import { IGenericErrors } from "../interfaces/IGenericErrors.sol";
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
    // Genesis DA stream position: the genesis dataRollingHash is the empty accumulator
    // (no chunks folded yet), and the genesis offset is 0 (fresh start). The sealed genesis
    // position commitment is keccak256(genesisDataRollingHash || 0).
    bytes32 genesisDataRollingHash = EMPTY_HASH;
    _dataRollingHashExists[genesisDataRollingHash] = DATA_ROLLING_HASH_EXISTS_DEFAULT_VALUE;
    bytes32 genesisPositionCommitment = _computePositionCommitment(genesisDataRollingHash, 0);

    __LinethRollup_init(_initializationData, genesisPositionCommitment);
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
   * @notice Bridges the last finalized 3-arg shnarf into the blob-spanning dataRollingHash model.
   * @dev This function is a reinitializer and can only be called once per version. Should be called
   *   using an upgradeAndCall transaction to the ProxyAdmin for live-chain (in-place) upgrades.
   * @dev Path-selection rule for the (previously undesigned) 3-arg-shnarf -> 2-arg-dataRollingHash
   *   transition: the slot that held the plain finalized shnarf is reinterpreted as the previous
   *   end dataRollingHash, and is resealed as the initial position commitment with offset 0
   *   (fresh-start). The previous shnarf value is captured BEFORE it is overwritten so the bridge
   *   is one-way. After this, the first post-upgrade finalization supplies
   *   (prevDataRollingHash = bridged value, prevOffset = 0) and takes the fresh-start branch
   *   (startOffset == 0), anchoring new dataRollingHashes from the bridged parent.
   * @dev The caller MUST supply the exact current value of `currentFinalizedShnarf` so the bridge
   *   reverts if the live state has drifted from what governance approved.
   * @param _currentFinalizedShnarf The current finalized 3-arg shnarf value to bridge.
   */
  function reinitializeLineaRollupV10(bytes32 _currentFinalizedShnarf) external reinitializer(10) nonReentrant {
    require(_currentFinalizedShnarf != EMPTY_HASH, IGenericErrors.ZeroHashNotAllowed());
    require(
      currentFinalizedShnarf == _currentFinalizedShnarf,
      BridgedShnarfMismatch(_currentFinalizedShnarf, currentFinalizedShnarf)
    );

    // Reinterpret the last finalized shnarf as the previous end dataRollingHash and anchor it so
    // post-upgrade submissions can chain from it.
    _dataRollingHashExists[_currentFinalizedShnarf] = DATA_ROLLING_HASH_EXISTS_DEFAULT_VALUE;

    // Seal the bridged position (offset 0 == fresh start) into the position-commitment slot.
    currentFinalizedShnarf = _computePositionCommitment(_currentFinalizedShnarf, 0);

    emit LineaRollupVersionChanged(bytes8("9.0"), bytes8("10.0"));
  }
}
