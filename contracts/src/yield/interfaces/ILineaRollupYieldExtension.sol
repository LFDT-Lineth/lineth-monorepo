// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.33;

import { IL1MessageService } from "../../messaging/l1/interfaces/IL1MessageService.sol";

/**
 * @title Native yield extension module for the Linea L1MessageService.
 * @author Consensys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
interface ILineaRollupYieldExtension {
  /**
   * @notice Emitted when ETH send from an authorized funder.
   * @param amount Donation amount.
   */
  event FundingReceived(uint256 amount);

  /**
   * @notice Emitted when a YieldManager address is set.
   * @param oldYieldManagerAddress The previous YieldManager address.
   * @param newYieldManagerAddress The new YieldManager address.
   */
  event YieldManagerChanged(address oldYieldManagerAddress, address newYieldManagerAddress);

  /**
   * @dev Thrown when the caller is not the YieldManager.
   */
  error CallerIsNotYieldManager();

  /**
   * @dev Thrown when an LST withdrawal is attempted while L1MessageService still has sufficient balance to covers the claim.
   */
  error LSTWithdrawalRequiresDeficit();

  /**
   * @dev Thrown when an LST withdrawal is attempted and the caller is not the recipient.
   */
  error CallerNotLSTWithdrawalRecipient();

  /**
   * @notice Report native yield earned for L2 distribution by emitting a synthetic `MessageSent` event.
   * @dev Callable only by the registered YieldManager.
   * @param _amount The net earned yield.
   * @param _l2YieldRecipient L2 account that the reported yield will be distributed to.
   */
  function reportNativeYield(uint256 _amount, address _l2YieldRecipient) external;

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
    IL1MessageService.ClaimMessageWithProofParams calldata _params,
    address _yieldProvider
  ) external;

  /**
   * @notice Returns whether the transient LST withdrawal flag is currently toggled.
   * @dev The flag is toggled during `claimMessageWithProofAndWithdrawLST` so the YieldManager can reject
   *      cross-chain invoked calls from `claimMessageWithProof` that lacks appropriate safeguards for LST minting.
   */
  function isWithdrawLSTAllowed() external view returns (bool);

  /**
   * @notice Transfer ETH to the registered YieldManager.
   * @dev YIELD_PROVIDER_STAKING_ROLE is required to execute.
   * @dev Enforces that, after transfer, the L1MessageService balance remains ≥ the configured effective minimum reserve.
   * @param _amount Amount of ETH to transfer.
   */
  function transferFundsForNativeYield(uint256 _amount) external;

  /**
   * @notice Send ETH to this contract.
   * @dev Accepts both permissionless donations and YieldManager withdrawals.
   */
  function fund() external payable;

  /**
   * @notice Set YieldManager address.
   * @dev SET_YIELD_MANAGER_ROLE is required to execute.
   * @dev If funds still exist on old YieldManager, it can still be withdrawn.
   * @param _newYieldManager YieldManager address.
   */
  function setYieldManager(address _newYieldManager) external;
}
