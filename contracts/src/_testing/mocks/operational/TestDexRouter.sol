// SPDX-License-Identifier: Apache-2.0 OR MIT
pragma solidity 0.8.33;

import { TestERC20 } from "../tokens/TestERC20.sol";

interface ISwapRouterV3 {
  struct ExactInputSingleParams {
    address tokenIn;
    address tokenOut;
    int24 tickSpacing;
    address recipient;
    uint256 deadline;
    uint256 amountIn;
    uint256 amountOutMinimum;
    uint160 sqrtPriceLimitX96;
  }

  function exactInputSingle(ExactInputSingleParams calldata params) external payable returns (uint256 amountOut);
}

contract TestDexRouter {
  struct ExactInputSingleParams {
    address tokenIn;
    address tokenOut;
    int24 tickSpacing;
    address recipient;
    uint256 deadline;
    uint256 amountIn;
    uint256 amountOutMinimum;
    uint160 sqrtPriceLimitX96;
  }

  function exactInputSingle(ExactInputSingleParams calldata params) external payable returns (uint256 amountOut) {
    amountOut = params.amountIn * 2;
    TestERC20(params.tokenOut).mint(params.recipient, amountOut);
  }

  function zeroOutputExactInputSingle(ExactInputSingleParams calldata) external payable returns (uint256 amountOut) {
    amountOut = 0;
  }
}
