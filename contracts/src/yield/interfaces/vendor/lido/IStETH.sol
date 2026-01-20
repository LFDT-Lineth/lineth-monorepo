// SPDX-FileCopyrightText: 2025 Lido <info@lido.fi>
// SPDX-License-Identifier: GPL-3.0

pragma solidity >=0.8.0;

interface IStETH {
  function getPooledEthBySharesRoundUp(uint256 _sharesAmount) external view returns (uint256 etherAmount);
  function getSharesByPooledEth(uint256 _ethAmount) external view returns (uint256);
}
