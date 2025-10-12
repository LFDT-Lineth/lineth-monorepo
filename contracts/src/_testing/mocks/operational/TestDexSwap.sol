// SPDX-License-Identifier: Apache-2.0 OR MIT
pragma solidity 0.8.30;

import { V3DexSwap } from "../../../operational/V3DexSwap.sol";

contract TestDexSwap is V3DexSwap {
  error TestRevertFromSwap();

  constructor(
    address _router,
    address _wethToken,
    address _lineaToken,
    uint24 _poolTickSpacing
  ) V3DexSwap(_router, _wethToken, _lineaToken, _poolTickSpacing) {}

  function testRevertSwap(uint256, uint256, uint160) external payable returns (uint256) {
    revert TestRevertFromSwap();
  }

  function testZeroAmountOutSwap(uint256, uint256, uint160) external payable returns (uint256) {
    return 0;
  }
}
