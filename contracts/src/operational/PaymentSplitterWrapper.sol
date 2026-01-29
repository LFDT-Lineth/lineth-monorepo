// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { PaymentSplitter } from "@openzeppelin/contracts/finance/PaymentSplitter.sol";

/**
 * @title PaymentSplitterWrapper
 * @dev Wrapper contract for OpenZeppelin's PaymentSplitter to enable Hardhat compilation.
 * This contract simply inherits from PaymentSplitter and passes constructor arguments through.
 * The deployed contract is functionally identical to deploying PaymentSplitter directly.
 */
contract PaymentSplitterWrapper is PaymentSplitter {
  /**
   * @notice Initializes the PaymentSplitterWrapper with the given payees and their corresponding shares.
   * @dev Passes constructor arguments through to the OpenZeppelin PaymentSplitter.
   * All addresses in `_payees` must be non-zero. Both arrays must have the same non-zero length,
   * and there must be no duplicates in `_payees`.
   * @param _payees The addresses of the payees to be added.
   * @param _shares The number of shares each payee owns, corresponding by index to `_payees`.
   */
  constructor(address[] memory _payees, uint256[] memory _shares) payable PaymentSplitter(_payees, _shares) {}
}
