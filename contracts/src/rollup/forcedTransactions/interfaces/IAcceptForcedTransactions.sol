// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.33;

/**
 * @title Interface to manage forced transaction storage functions.
 * @author Consensys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
interface IAcceptForcedTransactions {
  /**
   * @notice Emitted when the forced transaction fee is set.
   * @param forcedTransactionFeeInWei The forced transaction fee in wei.
   */
  event ForcedTransactionFeeSet(uint256 forcedTransactionFeeInWei);

  /**
   * @notice Emitted when the address filter is set.
   * @param oldAddressFilter The old address filter.
   * @param newAddressFilter The new address filter.
   */
  event AddressFilterChanged(address oldAddressFilter, address newAddressFilter);

  /**
   * @notice Emitted when a forced transaction is added.
   * @param forcedTransactionNumber The indexed forced transaction number.
   * @param from The recovered signer's from address.
   * @param blockNumberDeadline The maximum expected L2 block number processing will occur by.
   * @param forcedTransactionRollingHash The computed rolling Mimc based hash.
   * @param rlpEncodedSignedTransaction The RLP encoded type 02 transaction payload including signature.
   */
  event ForcedTransactionAdded(
    uint256 indexed forcedTransactionNumber,
    address indexed from,
    uint256 blockNumberDeadline,
    bytes32 forcedTransactionRollingHash,
    bytes rlpEncodedSignedTransaction
  );

  /**
   * @dev Thrown when another forced transaction is expected on the computed L2 block or the previous block number is higher than the submitted one.
   */
  error ForcedTransactionExistsForBlockOrIsTooLow(uint256 blockNumber);

  /**
   * @dev Thrown when a rolling hash is missing for a forced transaction number.
   */
  error MissingRollingHashForForcedTransactionNumber(uint256 forcedTransactionNumber);

  /**
   * @notice Provides state fields for forced transactions.
   * @return finalizedState The last finalized state hash.
   * @return previousForcedTransactionRollingHash The previous forced transaction rolling hash.
   * @return previousForcedTransactionBlockDeadline The previous forced transaction block deadline.
   * @return currentFinalizedL2BlockNumber The current finalized L2 block number.
   * @return forcedTransactionFeeAmount The forced transaction fee amount.
   */
  function getRequiredForcedTransactionFields()
    external
    returns (
      bytes32 finalizedState,
      bytes32 previousForcedTransactionRollingHash,
      uint256 previousForcedTransactionBlockDeadline,
      uint256 currentFinalizedL2BlockNumber,
      uint256 forcedTransactionFeeAmount
    );

  /**
   * @notice Stores forced transaction details required for proving feedback loop.
   * @dev FORCED_TRANSACTION_SENDER_ROLE is required to store a forced transaction.
   * @dev The forced transaction number is incremented for the next transaction post storage.
   * @dev The forced transaction fee is sent in the same transaction and the gateway will revert if it is not met.
   * @param _forcedTransactionRollingHash The rolling hash for all the forced transaction fields.
   * @param _from The recovered signer's from address.
   * @param _blockNumberDeadline The maximum expected L2 block number processing will occur by.
   * @param _rlpEncodedSignedTransaction The RLP encoded type 02 transaction payload including signature.
   */
  function storeForcedTransaction(
    bytes32 _forcedTransactionRollingHash,
    address _from,
    uint256 _blockNumberDeadline,
    bytes calldata _rlpEncodedSignedTransaction
  ) external payable;

  /**
   * @notice Sets the forced transaction fee.
   * @dev Only callable by an account with the FORCED_TRANSACTION_FEE_SETTER_ROLE.
   * @param _forcedTransactionFeeInWei The forced transaction fee in wei.
   */
  function setForcedTransactionFee(uint256 _forcedTransactionFeeInWei) external;

  /**
   * @notice Sets the address filter.
   * @dev Only callable by an account with the SET_ADDRESS_FILTER_ROLE.
   * @param _addressFilter The address filter.
   */
  function setAddressFilter(address _addressFilter) external;
}
