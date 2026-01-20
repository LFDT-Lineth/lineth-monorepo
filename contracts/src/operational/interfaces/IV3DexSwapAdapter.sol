// SPDX-License-Identifier: Apache-2.0 OR MIT
pragma solidity 0.8.33;

/**
 * @title Interface for the V3DexSwapAdapter contract.
 * @dev A contract for swapping tokens on the V3 decentralized exchange.
 * @author Consensys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
interface IV3DexSwapAdapter {
  /**
   * @dev Thrown when no ETH is sent with the swap call.
   */
  error NoEthSent();

  /**
   * @dev Thrown when a parameter is the zero address.
   */
  error ZeroAddressNotAllowed();

  /**
   * @dev Thrown when the tick spacing is zero.
   */
  error ZeroTickSpacingNotAllowed();

  /**
   * @dev Thrown when the minimum LINEA out parameter is zero.
   */
  error ZeroMinLineaOutNotAllowed();

  /**
   * @dev Thrown when the deadline is in the past. (< block.timestamp)
   */
  error DeadlineInThePast();

  /**
   * @dev Thrown when insufficient LINEA tokens are received from the DEX swap.
   * @param expectedMinimum The expected minimum number of LINEA tokens to be received.
   * @param actualReceived The actual number of LINEA tokens received.
   */
  error InsufficientLineaTokensReceived(uint256 expectedMinimum, uint256 actualReceived);

  /** @notice Swap ETH into LINEA.
   * @notice Emitted when the V3DexAdapter contract is initialized.
   * @param router The address of the Router contract.
   * @param wethToken The address of the WETH token contract.
   * @param lineaToken The address of the LINEA token contract.
   * @param poolTickSpacing Tick spacing of the pool.
   */
  event V3DexSwapAdapterInitialized(address router, address wethToken, address lineaToken, int24 poolTickSpacing);

  /** @notice Swap ETH into LINEA tokens.
   * @dev The msg.sender will be the recipient of the LINEA tokens, and WETH is swapped 1:1 with `msg.value`.
   * @dev No ETH is kept in the contract after the swap due to exactInputSingle swapping.
   * @param _minLineaOut Minimum number of LINEA tokens to receive (slippage protection).
   * @param _deadline Time after which the transaction will revert if not yet processed.
   * @return amountOut The amount of LINEA tokens received from the swap.
   */
  function swap(uint256 _minLineaOut, uint256 _deadline) external payable returns (uint256 amountOut);
}
