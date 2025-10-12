// SPDX-License-Identifier: Apache-2.0 OR MIT
pragma solidity 0.8.30;

/**
 * @title Interface for the V3DexSwap contract.
 * @dev A contract for swapping tokens on the V3 decentralized exchange.
 * @author Consensys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
interface IV3DexSwap {
  /**
   * @dev Thrown when no ETH is sent with the swap call.
   */
  error NoEthSend();

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

  /** @notice Swap ETH into LINEA.
   * @param _minLineaOut Minimum number of LINEA tokens to receive (slippage protection).
   * @param _deadline Time after which the transaction will revert if not yet processed.
   * @param _sqrtPriceLimitX96 Price limit of the swap as a Q64.96 value.
   */
  function swap(
    uint256 _minLineaOut,
    uint256 _deadline,
    uint160 _sqrtPriceLimitX96
  ) external payable returns (uint256 amountOut);
}
