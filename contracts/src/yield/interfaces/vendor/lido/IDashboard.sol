// SPDX-FileCopyrightText: 2025 Lido <info@lido.fi>
// SPDX-License-Identifier: GPL-3.0

import { IStakingVault } from "./IStakingVault.sol";
import { ICommonVaultOperations } from "./ICommonVaultOperations.sol";

// See contracts/COMPILERS.md
pragma solidity >=0.8.0;

/**
 * @title Dashboard
 * @notice This contract is a UX-layer for StakingVault and meant to be used as its owner.
 * This contract improves the vault UX by bundling all functions from the StakingVault and VaultHub
 * in this single contract. It provides administrative functions for managing the StakingVault,
 * including funding, withdrawing, minting, burning, and rebalancing operations.
 */
interface IDashboard is ICommonVaultOperations {
  function stakingVault() external view returns (IStakingVault);

  function totalValue() external view returns (uint256);

  function liabilityShares() external view returns (uint256);

  function withdrawableValue() external view returns (uint256);

  function voluntaryDisconnect() external;

  function abandonDashboard(address _newOwner) external;

  function mintStETH(address _recipient, uint256 _amountOfStETH) external payable;

  function rebalanceVaultWithShares(uint256 _shares) external;

  function rebalanceVaultWithEther(uint256 _ether) external payable;

  function accruedFee() external view returns (uint256);

  function disburseFee() external;

  function reconnectToVaultHub() external;

  function transferVaultOwnership(address _newOwner) external;

  function obligations() external view returns (uint256 sharesToBurn, uint256 feesToSettle);
}
