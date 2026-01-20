// SPDX-License-Identifier: Apache-2.0 OR MIT
pragma solidity 0.8.33;

import { IERC20 } from "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import { IWETH9 } from "../../../operational/interfaces/IWETH9.sol";
import { TestDexRouter } from "./TestDexRouter.sol";
import { V3DexSwapAdapter } from "../../../operational/V3DexSwapAdapter.sol";

contract TestDexSwapAdapter is V3DexSwapAdapter {
  error TestRevertFromSwap();

  constructor(
    address _router,
    address _wethToken,
    address _lineaToken,
    int24 _poolTickSpacing
  ) V3DexSwapAdapter(_router, _wethToken, _lineaToken, _poolTickSpacing) {}

  function testRevertSwap(uint256, uint256) external payable returns (uint256) {
    revert TestRevertFromSwap();
  }

  /** @notice Swap ETH into LINEA tokens.
   * @dev The msg.sender will be the recipient of the LINEA tokens, and WETH is swapped 1:1 with `msg.value`.
   * @dev No ETH is kept in the contract after the swap due to exactInputSingle swapping.
   * @param _minLineaOut Minimum number of LINEA tokens to receive (slippage protection).
   * @param _deadline Time after which the transaction will revert if not yet processed.
   */
  function testSwapInsufficientLineaTokensReceived(
    uint256 _minLineaOut,
    uint256 _deadline
  ) external payable returns (uint256 amountOut) {
    require(msg.value > 0, NoEthSent());
    require(_deadline > block.timestamp, DeadlineInThePast());
    require(_minLineaOut > 0, ZeroMinLineaOutNotAllowed());

    IWETH9(WETH_TOKEN).deposit{ value: msg.value }();
    IWETH9(WETH_TOKEN).approve(ROUTER, msg.value);

    uint256 balanceBefore = IERC20(LINEA_TOKEN).balanceOf(msg.sender);

    amountOut = TestDexRouter(ROUTER).zeroOutputExactInputSingle(
      TestDexRouter.ExactInputSingleParams({
        tokenIn: WETH_TOKEN,
        tokenOut: LINEA_TOKEN,
        tickSpacing: POOL_TICK_SPACING,
        recipient: msg.sender,
        deadline: _deadline,
        amountIn: msg.value,
        amountOutMinimum: _minLineaOut,
        sqrtPriceLimitX96: 0
      })
    );

    uint256 received = IERC20(LINEA_TOKEN).balanceOf(msg.sender) - balanceBefore;
    require(received >= _minLineaOut, InsufficientLineaTokensReceived(_minLineaOut, received));
  }
}
