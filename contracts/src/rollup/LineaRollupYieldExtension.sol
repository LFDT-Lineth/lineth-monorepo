// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.33;

import { LineaRollupBase } from "./LineaRollupBase.sol";
import { ILineaRollupYieldExtension } from "../yield/interfaces/ILineaRollupYieldExtension.sol";
import { IYieldManager } from "../yield/interfaces/IYieldManager.sol";
import { MessageHashing } from "../messaging/libraries/MessageHashing.sol";
import { ErrorUtils } from "../libraries/ErrorUtils.sol";

/**
 * @title Native yield extension module for the Linea L1MessageService.
 * @author Consensys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
abstract contract LineaRollupYieldExtension is LineaRollupBase, ILineaRollupYieldExtension {
  /// @notice The role required to send ETH to the YieldManager.
  bytes32 public constant YIELD_PROVIDER_STAKING_ROLE = keccak256("YIELD_PROVIDER_STAKING_ROLE");

  /// @notice The role required to set the YieldManager address.
  bytes32 public constant SET_YIELD_MANAGER_ROLE = keccak256("SET_YIELD_MANAGER_ROLE");

  bool private transient IS_WITHDRAW_LST_ALLOWED;

  /// @dev To consider - could this be an immutable variable instead? Do we expect YieldManager instance to change at a different cadence than LineaRollup upgrades?
  /// @custom:storage-location erc7201:linea.storage.LineaRollupYieldExtensionStorage
  struct LineaRollupYieldExtensionStorage {
    address _yieldManager;
  }

  // keccak256(abi.encode(uint256(keccak256("linea.storage.LineaRollupYieldExtensionStorage")) - 1)) & ~bytes32(uint256(0xff))
  bytes32 private constant LineaRollupYieldExtensionStorageLocation =
    0x594904a11ae10ad7613c91ac3c92c7c3bba397934d377ce6d3e0aaffbc17df00;

  function _storage() private pure returns (LineaRollupYieldExtensionStorage storage $) {
    assembly {
      $.slot := LineaRollupYieldExtensionStorageLocation
    }
  }

  /// @notice The address of the YieldManager.
  function yieldManager() public view returns (address) {
    return _storage()._yieldManager;
  }

  /**
   * @notice Initialises the LineaRollupYieldExtension.
   * @param _yieldManager YieldManager address.
   */
  function __LineaRollupYieldExtension_init(address _yieldManager) internal onlyInitializing {
    _setYieldManager(_yieldManager);
  }

  function isWithdrawLSTAllowed() public view virtual returns (bool) {
    return IS_WITHDRAW_LST_ALLOWED;
  }

  /**
   * @notice Transfer ETH to the registered YieldManager.
   * @dev YIELD_PROVIDER_STAKING_ROLE is required to execute.
   * @dev Enforces that, after transfer, the L1MessageService balance remains ≥ the configured effective minimum reserve.
   * @param _amount Amount of ETH to transfer.
   */
  function transferFundsForNativeYield(
    uint256 _amount
  ) public virtual whenTypeAndGeneralNotPaused(PauseType.NATIVE_YIELD_STAKING) onlyRole(YIELD_PROVIDER_STAKING_ROLE) {
    IYieldManager(yieldManager()).receiveFundsFromReserve{ value: _amount }();
  }

  /**
   * @notice Send ETH to this contract.
   * @dev Accepts both permissionless donations and YieldManager withdrawals.
   */
  function fund() external payable virtual {
    if (msg.value == 0) revert NoEthSent();
    emit FundingReceived(msg.value);
  }

  /**
   * @notice Set YieldManager address.
   * @dev SET_YIELD_MANAGER_ROLE is required to execute.
   * @dev If funds still exist on old YieldManager, it can still be withdrawn.
   * @param _newYieldManager YieldManager address.
   */
  function setYieldManager(address _newYieldManager) external onlyRole(SET_YIELD_MANAGER_ROLE) {
    _setYieldManager(_newYieldManager);
  }

  /**
   * @notice Set YieldManager address.
   * @dev If funds still exist on old YieldManager, it can still be withdrawn.
   * @param _newYieldManager YieldManager address.
   */
  function _setYieldManager(address _newYieldManager) internal {
    ErrorUtils.revertIfZeroAddress(_newYieldManager);
    emit YieldManagerChanged(_storage()._yieldManager, _newYieldManager);
    _storage()._yieldManager = _newYieldManager;
  }

  /**
   * @notice Report native yield earned for L2 distribution by emitting a synthetic `MessageSent` event.
   * @dev Callable only by the registered YieldManager.
   * @param _amount The net earned yield.
   */
  function reportNativeYield(
    uint256 _amount,
    address _l2YieldRecipient
  ) public virtual whenTypeAndGeneralNotPaused(PauseType.L1_L2) {
    if (msg.sender != yieldManager()) {
      revert CallerIsNotYieldManager();
    }
    require(_l2YieldRecipient != address(0), ZeroAddressNotAllowed());

    uint256 messageNumber = nextMessageNumber++;
    bytes32 messageHash = MessageHashing._hashMessageWithEmptyCalldata(
      address(this),
      _l2YieldRecipient,
      0,
      _amount,
      messageNumber
    );

    _addRollingHash(messageNumber, messageHash);

    emit MessageSent(msg.sender, _l2YieldRecipient, 0, _amount, messageNumber, hex"", messageHash);
  }

  /**
   * @notice Claims a cross-chain message using a Merkle proof, and withdraws LST from the specified yield provider
   *         when the L1MessageService balance is insufficient to fulfill delivery.
   *
   * @dev Reverts if the L1MessageService has sufficient balance to fulfill the message delivery.
   * @dev Differences from `claimMessageWithProof`:
   *      - Does not deliver the message payload to the recipient, as the L1MessageService lacks sufficient balance.
   *      - Does not provide a refund of the message fee.
   * @dev Temporarily enables an alternate call path by toggling the `IS_WITHDRAW_LST_ALLOWED` flag,
   *      which is unavailable via `claimMessageWithProof`.
   * @dev Reverts with `L2MerkleRootDoesNotExist` if no Merkle tree exists at the specified depth.
   * @dev Reverts with `ProofLengthDifferentThanMerkleDepth` if the provided proof size does not match the tree depth.
   *
   * @param _params Collection of claim data with proof and supporting data.
   * @param _yieldProvider The yield provider address to withdraw LST from.
   */
  function claimMessageWithProofAndWithdrawLST(
    ClaimMessageWithProofParams calldata _params,
    address _yieldProvider
  ) public virtual nonReentrant {
    if (_params.value <= address(this).balance) {
      revert LSTWithdrawalRequiresDeficit();
    }
    if (msg.sender != _params.to) {
      revert CallerNotLSTWithdrawalRecipient();
    }
    bytes32 messageLeafHash = _validateAndConsumeMessageProof(_params);
    IS_WITHDRAW_LST_ALLOWED = true;
    IYieldManager(yieldManager()).withdrawLST(_yieldProvider, _params.value, _params.to);
    IS_WITHDRAW_LST_ALLOWED = false;
    emit MessageClaimed(messageLeafHash);
  }
}
