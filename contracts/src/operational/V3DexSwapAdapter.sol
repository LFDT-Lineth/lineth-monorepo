// SPDX-License-Identifier: Apache-2.0 OR MIT
pragma solidity 0.8.33;

import { IERC20 } from "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import { IV3DexSwapAdapter } from "./interfaces/IV3DexSwapAdapter.sol";
import { ISwapRouterV3 } from "./interfaces/ISwapRouterV3.sol";

/**
 * @title V3DexSwapAdapter.
 * @dev A contract for swapping tokens on a decentralized exchange.
 * @author Consensys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
contract V3DexSwapAdapter is IV3DexSwapAdapter {
  /// @notice Tick spacing of the pool.
  int24 public immutable POOL_TICK_SPACING;
  /// @notice Address of the Swap Router contract.
  address public immutable ROUTER;
  /// @notice Address of the WETH token contract.
  address public immutable WETH_TOKEN;
  /// @notice Address of the LINEA token contract.
  address public immutable LINEA_TOKEN;

  /**
   * @dev Initializes the contract with the given parameters.
   * @dev The expect pools use int24.
   * @param _router Address of the Router contract.
   * @param _wethToken Address of the WETH token contract.
   * @param _lineaToken Address of the LINEA token contract.
   * @param _poolTickSpacing Tick spacing of the pool.
   */
  constructor(address _router, address _wethToken, address _lineaToken, int24 _poolTickSpacing) {
    require(_router != address(0), ZeroAddressNotAllowed());
    require(_wethToken != address(0), ZeroAddressNotAllowed());
    require(_lineaToken != address(0), ZeroAddressNotAllowed());
    require(_poolTickSpacing > 0, ZeroTickSpacingNotAllowed());

    ROUTER = _router;
    WETH_TOKEN = _wethToken;
    LINEA_TOKEN = _lineaToken;
    POOL_TICK_SPACING = _poolTickSpacing;

    emit V3DexSwapAdapterInitialized(_router, _wethToken, _lineaToken, _poolTickSpacing);
  }

  /** @notice Swap ETH into LINEA tokens.
   * @dev The msg.sender will be the recipient of the LINEA tokens, and WETH is swapped 1:1 with `msg.value`.
   * @dev No ETH is kept in the contract after the swap due to exactInputSingle swapping.
   * @param _minLineaOut Minimum number of LINEA tokens to receive (slippage protection).
   * @param _deadline Time after which the transaction will revert if not yet processed.
   * @return amountOut The amount of LINEA tokens received from the swap.
   */
  function swap(uint256 _minLineaOut, uint256 _deadline) external payable returns (uint256 amountOut) {
    require(msg.value > 0, NoEthSent());
    require(_deadline >= block.timestamp, DeadlineInThePast());
    require(_minLineaOut > 0, ZeroMinLineaOutNotAllowed());

    uint256 tokenBalanceBefore = IERC20(LINEA_TOKEN).balanceOf(msg.sender);

    amountOut = ISwapRouterV3(ROUTER).exactInputSingle{ value: msg.value }(
      ISwapRouterV3.ExactInputSingleParams({
        tokenIn: WETH_TOKEN,
        tokenOut: LINEA_TOKEN,
        tickSpacing: POOL_TICK_SPACING,
        recipient: msg.sender,
        deadline: _deadline,
        amountIn: msg.value,
        amountOutMinimum: _minLineaOut,
        /// @dev Setting to 0 because _minLineaOut handles slippage protection.
        sqrtPriceLimitX96: 0
      })
    );

    uint256 tokensReceived = IERC20(LINEA_TOKEN).balanceOf(msg.sender) - tokenBalanceBefore;
    require(tokensReceived >= _minLineaOut, InsufficientLineaTokensReceived(_minLineaOut, tokensReceived));
  }
}
