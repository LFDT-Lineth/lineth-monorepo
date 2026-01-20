// SPDX-FileCopyrightText: 2025 Lido <info@lido.fi>
// SPDX-License-Identifier: GPL-3.0

// See contracts/COMPILERS.md
// solhint-disable-next-line lido/fixed-compiler-version
pragma solidity >=0.8.0;

interface IVaultHub {
  function settleLidoFees(address _vault) external;
  function isVaultConnected(address _vault) external view returns (bool);
  function isPendingDisconnect(address _vault) external view returns (bool);
}
