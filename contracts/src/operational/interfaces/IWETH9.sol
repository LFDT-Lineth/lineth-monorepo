// SPDX-License-Identifier: Apache-2.0 OR MIT
pragma solidity 0.8.33;

import { IERC20 } from "@openzeppelin/contracts/token/ERC20/IERC20.sol";

/**
 * @title WETH9 interface.
 * @author Consensys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
interface IWETH9 is IERC20 {
  /**
   * @notice Deposits ETH into the WETH9 contract.
   */
  function deposit() external payable;
}
