// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.33;

import { IMessageService } from "../../../messaging/interfaces/IMessageService.sol";
import { IGenericErrors } from "../../../interfaces/IGenericErrors.sol";
import { LineaRollupPauseManager } from "../../../security/pausing/LineaRollupPauseManager.sol";
import { L1MessageManager } from "../../../messaging/l1/L1MessageManager.sol";

contract MockMessageServiceV2 is L1MessageManager, IMessageService, LineaRollupPauseManager, IGenericErrors {
  address internal messageSender = address(0);
  uint256 public nextMessageNumber = 1;

  /**
   * @notice Adds a message for sending cross-chain and emits MessageSent.
   * @dev The message number is preset (nextMessageNumber) and only incremented at the end if successful for the next caller.
   * @dev This function should be called with a msg.value = _value + _fee. The fee will be paid on the destination chain.
   * @param _to The address the message is intended for.
   * @param _fee The fee being paid for the message delivery.
   * @param _calldata The calldata to pass to the recipient.
   */
  function sendMessage(
    address _to,
    uint256 _fee,
    bytes calldata _calldata
  ) external payable whenTypeAndGeneralNotPaused(PauseType.L1_L2) {
    if (_to == address(0)) {
      revert ZeroAddressNotAllowed();
    }

    if (_fee > msg.value) {
      revert ValueSentTooLow();
    }

    uint256 messageNumber = nextMessageNumber;
    uint256 valueSent = msg.value - _fee;

    bytes32 messageHash = keccak256(abi.encode(msg.sender, _to, _fee, valueSent, messageNumber, _calldata));

    nextMessageNumber++;

    emit MessageSent(msg.sender, _to, _fee, valueSent, messageNumber, _calldata, messageHash);
  }

  // When called within the context of the delivered call returns the sender from the other layer
  // otherwise returns the zero address
  function sender() external view returns (address) {
    return messageSender;
  }

  // Placeholder
  function claimMessage(
    address _from,
    address _to,
    uint256 _fee,
    uint256 _value,
    address payable _feeRecipient,
    bytes calldata _calldata,
    uint256 _nonce
  ) external {}
}
