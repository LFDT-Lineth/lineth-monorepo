// SPDX-FileCopyrightText: 2025 Lido <info@lido.fi>
// SPDX-License-Identifier: GPL-3.0

pragma solidity >=0.8.0;

// Shared interface between IDashboard and IStakingVault
interface ICommonVaultOperations {
  /**
   * @notice Funds the staking vault with ether
   */
  function fund() external payable;

  /**
   * @notice Withdraws ether from the staking vault to a recipient
   * @param _recipient Address of the recipient
   * @param _ether Amount of ether to withdraw
   */
  function withdraw(address _recipient, uint256 _ether) external;

  /**
   * @notice Pauses beacon chain deposits on the StakingVault.
   */
  function pauseBeaconChainDeposits() external;

  /**
   * @notice Resumes beacon chain deposits on the StakingVault.
   */
  function resumeBeaconChainDeposits() external;

  /**
   * @notice Initiates a withdrawal from validator(s) on the beacon chain using EIP-7002 triggerable withdrawals
   *         Both partial withdrawals (disabled for if vault is unhealthy) and full validator exits are supported.
   * @param _pubkeys Concatenated validator external keys (48 bytes each).
   * @param _amounts Withdrawal amounts in gwei for each validator key and must match _pubkeys length.
   *         Set amount to 0 for a full validator exit.
   *         For partial withdrawals, amounts will be trimmed to keep MIN_ACTIVATION_BALANCE on the validator to avoid deactivation
   * @param _refundRecipient Address to receive any fee refunds. Must be non-zero as StakingVault will revert otherwise.
   * @dev    A withdrawal fee must be paid via msg.value.
   *         Use `StakingVault.calculateValidatorWithdrawalFee()` to determine the required fee for the current block.
   */
  function triggerValidatorWithdrawals(
    bytes calldata _pubkeys,
    uint64[] calldata _amounts,
    address _refundRecipient
  ) external payable;
}
