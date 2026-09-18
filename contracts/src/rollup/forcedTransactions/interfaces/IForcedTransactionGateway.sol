// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.33;

/**
 * @title Interface to manage forced transactions on L1.
 * @author Consensys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
interface IForcedTransactionGateway {
  /**
   * @notice Supporting data for the last finalized rollup state.
   * @param timestamp The last finalized timestamp.
   * @param messageNumber The L2 computed L1 message number.
   * @param messageRollingHash The L2 computed L1 message rolling hash.
   * @param forcedTransactionNumber The last finalized forced transaction processed on L2.
   * @param forcedTransactionRollingHash The last finalized forced transaction's rolling hash processed on L2.
   */
  struct LastFinalizedState {
    uint256 timestamp;
    uint256 messageNumber;
    bytes32 messageRollingHash;
    uint256 forcedTransactionNumber;
    bytes32 forcedTransactionRollingHash;
  }

  /**
   * @notice Supporting data for an EIP-1559 transaction.
   * @param nonce The nonce for the transaction belonging to the signer.
   * @param maxPriorityFeePerGas The max priority fee per gas.
   * @param maxFeePerGas The max fee per gas.
   * @param gasLimit The transaction's gas limit.
   * @param to The destination address of the transaction.
   * @param value The Ether value to transfer.
   * @param input The calldata input to send to the "to" address.
   * @param yParity The signature's yParity.
   * @param r The r portion of the signature.
   * @param s The s portion of the signature.
   */
  struct Eip1559Transaction {
    uint256 nonce;
    uint256 maxPriorityFeePerGas;
    uint256 maxFeePerGas;
    uint256 gasLimit;
    address to;
    uint256 value;
    bytes input;
    uint8 yParity;
    uint256 r;
    uint256 s;
  }

  /**
   * @notice Emitted when the useAddressFilter toggle state changes.
   * @param useAddressFilter The feature toggle enabled status.
   */
  event AddressFilterSet(bool useAddressFilter);

  /**
   * @dev Thrown when an address is on the address filter.
   */
  error AddressIsFiltered();

  /**
   * @dev Thrown when the max gas limit configured will be exceeded.
   */
  error MaxGasLimitExceeded();

  /**
   * @dev Thrown when the configured max gas limit is less than or equal to the configured min gas limit.
   */
  error MaxGasLimitLessThanOrEqualToMinGasLimit(uint256 minGasLimit, uint256 maxGasLimit);

  /**
   * @dev Thrown when the gas limit is below the configured minimum.
   */
  error GasLimitTooLow();

  /**
   * @dev Thrown when the input length on the transaction is too long.
   */
  error CalldataInputLengthLimitExceeded();

  /**
   * @dev Thrown when one of the gas fee parameters are zero (only checked when MINIMUM_BASE_GAS_FEE is non-zero).
   */
  error GasFeeParametersContainZero(uint256 maxFeePerGas, uint256 maxPriorityFeePerGas);

  /**
   * @dev Thrown when the transaction's maxFeePerGas is below the configured MINIMUM_BASE_GAS_FEE.
   *      Only checked when MINIMUM_BASE_GAS_FEE is non-zero.
   */
  error MaxFeePerGasLowerThanMinimumBaseGasFee(uint256 maxFeePerGas, uint256 minimumBaseGasFee);

  /**
   * @dev Thrown when the max priority fee per gas is higher than the max fee per gas.
   */
  error MaxPriorityFeePerGasHigherThanMaxFee(uint256 maxFeePerGas, uint256 maxPriorityFeePerGas);

  /**
   * @dev Thrown when the yParity is not 0 or 1.
   */
  error YParityGreaterThanOne(uint256 yParity);

  /**
   * @dev Thrown when finalization state does not match.
   */
  error FinalizationStateIncorrect(bytes32 expected, bytes32 value);

  /**
   * @dev Thrown when the toggle status requested is already set.
   */
  error AddressFilterAlreadySet(bool requestedExistingStatus);

  /**
   * @dev Thrown when the forced transaction fee is not met.
   */
  error ForcedTransactionFeeNotMet(uint256 expected, uint256 value);

  /**
   * @dev Thrown when a forced transaction has already been submitted in the current L1 block.
   */
  error ForcedTransactionAlreadySubmittedInBlock(uint256 blockNumber);

  /**
   * @notice Function to submit forced transactions.
   * @param _forcedTransaction The fields required for the transaction excluding chainId.
   * @param _lastFinalizedState The last finalized state validated to use the timestamp in block number calculation.
   */
  function submitForcedTransaction(
    Eip1559Transaction memory _forcedTransaction,
    LastFinalizedState memory _lastFinalizedState
  ) external payable;

  /**
   * @notice Function to toggle the usage of the address filter.
   * @dev Only callable by an account with the DEFAULT_ADMIN_ROLE.
   * @param _useAddressFilter Bool indicating whether or not to use the address filter.
   */
  function toggleUseAddressFilter(bool _useAddressFilter) external;
}
