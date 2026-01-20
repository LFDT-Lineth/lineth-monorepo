// SPDX-License-Identifier: Apache-2.0 OR MIT
pragma solidity 0.8.33;

/**
 * @title V3 Router interface.
 * @author Consensys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
interface ISwapRouterV3 {
  /**
   * @dev Parameters for the exact input single swap.
   * @param tokenIn The address of the input token.
   * @param tokenOut The address of the output token.
   * @param tickSpacing The tick spacing of the pool to swap in.
   * @param recipient The address to receive the output tokens.
   * @param deadline The time by which the transaction must be included to be valid.
   * @param amountIn The amount of input tokens to swap.
   * @param amountOutMinimum The minimum amount of output tokens that must be received for the
   * transaction not to revert.
   * @param sqrtPriceLimitX96 The Q64.96 sqrt price limit. If 0, no price limit.
   */
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

  /**
   * @notice Swaps `amountIn` of one token for as much as possible of another token.
   * @param params The parameters necessary for the swap, encoded as `ExactInputSingleParams` in calldata.
   * @return amountOut The amount of the received token.
   */
  function exactInputSingle(ExactInputSingleParams calldata params) external payable returns (uint256 amountOut);
}
