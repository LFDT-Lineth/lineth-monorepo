// SPDX-FileCopyrightText: 2025 Lido <info@lido.fi>
// SPDX-License-Identifier: GPL-3.0

// See contracts/COMPILERS.md
// solhint-disable-next-line lido/fixed-compiler-version
pragma solidity >=0.8.0;

import { ICommonVaultOperations } from "./ICommonVaultOperations.sol";

/**
 * @title IStakingVault
 * @author Lido
 * @notice Interface for the `StakingVault` contract
 */
interface IStakingVault is ICommonVaultOperations {
  function acceptOwnership() external;

  function ossify() external;

  function availableBalance() external view returns (uint256);

  function setDepositor(address _depositor) external;
  function stagedBalance() external view returns (uint256);
  function unstage(uint256 _ether) external;
  function transferOwnership(address _newOwner) external;
}
