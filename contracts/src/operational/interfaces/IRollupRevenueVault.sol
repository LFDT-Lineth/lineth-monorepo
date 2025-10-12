// SPDX-License-Identifier: Apache-2.0 OR MIT
pragma solidity 0.8.30;

/**
 * @title Interface for Rollup Revenue Vault Contract.
 * @notice Accepts rollup revenue, and performs burning operations.
 * @author Consensys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
interface IRollupRevenueVault {
  /**
   * @dev Thrown when a parameter is the zero address.
   */
  error ZeroAddressNotAllowed();

  /**
   * @dev Thrown when a timestamp is zero.
   */
  error ZeroTimestampNotAllowed();

  /**
   * @dev Thrown when the invoice transfer fails.
   */
  error InvoiceTransferFailed();

  /**
   * @dev Thrown when the burn of ETH fails.
   */
  error EthBurnFailed();

  /**
   * @dev Thrown when the timestamps are not in sequence.
   */
  error TimestampsNotInSequence();

  /**
   * @dev Thrown when the end timestamp is not greater than the start timestamp.
   */
  error EndTimestampMustBeGreaterThanStartTimestamp();

  /**
   * @dev Thrown when the invoice amount is zero.
   */
  error ZeroInvoiceAmount();

  /**
   * @dev Thrown when the invoice is in arrears.
   */
  error InvoiceInArrears();

  /**
   * @dev Thrown when the invoice date is too old.
   */
  error InvoiceDateTooOld();

  /**
   * @dev Thrown when the contract balance is insufficient.
   */
  error InsufficientBalance();

  /**
   * @dev Thrown when the provided address is the same as the existing address.
   */
  error ExistingAddressTheSame();

  /**
   * @dev Thrown when the DEX swap fails.
   */
  error DexSwapFailed();

  /**
   * @dev Thrown when zero LINEA tokens are received from the DEX swap.
   */
  error ZeroLineaTokensReceived();

  /**
   * @dev Emitted when an invoice is processed.
   * @dev If amountRequested < amountPaid, the difference is previous unpaid invoice amount.
   * @param receiver The address of the invoice receiver.
   * @param startTimestamp The start timestamp of the invoicing period.
   * @param endTimestamp The end timestamp of the invoicing period.
   * @param amountPaid The amount that was paid.
   * @param amountRequested The amount that was requested.
   */
  event InvoiceProcessed(
    address indexed receiver,
    uint256 indexed startTimestamp,
    uint256 indexed endTimestamp,
    uint256 amountPaid,
    uint256 amountRequested
  );

  /**
   * @dev Emitted when ETH is burnt, swapped, and bridged.
   * @param ethBurnt The amount of ETH that was burnt.
   * @param lineaTokensBridged The amount of LINEA tokens that were bridged.
   */
  event EthBurntSwappedAndBridged(uint256 ethBurnt, uint256 lineaTokensBridged);

  /**
   * @dev Emitted when the L1 LINEA token burner contract address is updated.
   * @param previousValue The previous L1 LINEA token burner contract address.
   * @param newValue The new L1 LINEA token burner contract address.
   */
  event L1LineaTokenBurnerUpdated(address previousValue, address newValue);

  /**
   * @dev Emitted when the DEX contract address is updated.
   * @param previousValue The previous DEX contract address.
   * @param newValue The new DEX contract address.
   */
  event DexUpdated(address previousValue, address newValue);

  /**
   * @dev Emitted when ETH is received.
   * @param amount The amount of ETH received.
   */
  event EthReceived(uint256 amount);

  /**
   * @dev Emitted when the invoice arrears are updated.
   * @param newInvoiceArrears The new invoice arrears value.
   * @param lastInvoiceDate The timestamp of the last invoice processed.
   */
  event InvoiceArrearsUpdated(uint256 newInvoiceArrears, uint256 lastInvoiceDate);

  /**
   * @dev Emitted when the invoice payment receiver is updated.
   * @param previousValue The previous invoice payment receiver address.
   * @param newValue The new invoice payment receiver address.
   */
  event InvoicePaymentReceiverUpdated(address previousValue, address newValue);
}
