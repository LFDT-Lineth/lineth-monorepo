// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.30;

/**
 * @title Interface declaring pre-existing cross-chain messaging functions, events and errors.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
interface IMessageService {
  /**
   * @notice Emitted when a message is sent.
   * @param _from The indexed sender address of the message (msg.sender).
   * @param _to The indexed intended recipient address of the message on the other layer.
   * @param _fee The fee being being paid to deliver the message to the recipient in Wei.
   * @param _value The value being sent to the recipient in Wei.
   * @param _nonce The unique message number.
   * @param _calldata The calldata being passed to the intended recipient when being called on claiming.
   * @param _messageHash The indexed hash of the message parameters.
   * @dev _calldata has the _ because calldata is a reserved word.
   * @dev We include the message hash to save hashing costs on the rollup.
   * @dev This event is used on both L1 and L2.
   */
  event MessageSent(
    address indexed _from,
    address indexed _to,
    uint256 _fee,
    uint256 _value,
    uint256 _nonce,
    bytes _calldata,
    bytes32 indexed _messageHash
  );

  /**
   * @notice Emitted when a message is claimed.
   * @param _messageHash The indexed hash of the message that was claimed.
   */
  event MessageClaimed(bytes32 indexed _messageHash);

  /**
   * @dev Thrown when fees are lower than the minimum fee.
   */
  error FeeTooLow();

  /**
   * @dev Thrown when the value sent is less than the fee.
   * @dev Value to forward on is msg.value - _fee.
   */
  error ValueSentTooLow();

  /**
   * @dev Thrown when the destination address reverts.
   */
  error MessageSendingFailed(address destination);

  /**
   * @dev Thrown when the recipient address reverts.
   */
  error FeePaymentFailed(address recipient);

  /**
   * @notice Sends a message for transporting from the given chain.
   * @dev This function should be called with a msg.value = _value + _fee. The fee will be paid on the destination chain.
   * @param _to The destination address on the destination chain.
   * @param _fee The message service fee on the origin chain.
   * @param _calldata The calldata used by the destination message service to call the destination contract.
   */
  function sendMessage(address _to, uint256 _fee, bytes calldata _calldata) external payable;

  /**
   * @notice Deliver a message to the destination chain.
   * @notice Is called by the Postman, dApp or end user.
   * @param _from The msg.sender calling the origin message service.
   * @param _to The destination address on the destination chain.
   * @param _value The value to be transferred to the destination address.
   * @param _fee The message service fee on the origin chain.
   * @param _feeRecipient Address that will receive the fees.
   * @param _calldata The calldata used by the destination message service to call/forward to the destination contract.
   * @param _nonce Unique message number.
   */
  function claimMessage(
    address _from,
    address _to,
    uint256 _fee,
    uint256 _value,
    address payable _feeRecipient,
    bytes calldata _calldata,
    uint256 _nonce
  ) external;

  /**
   * @notice Returns the original sender of the message on the origin layer.
   * @return originalSender The original sender of the message on the origin layer.
   */
  function sender() external view returns (address originalSender);
}
