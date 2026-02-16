// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.33;

import { L1MessageServiceBase } from "../L1MessageServiceBase.sol";
import { IClaimMessageV1 } from "../../l1/v1/interfaces/IClaimMessageV1.sol";
import { MessageHashing } from "../../libraries/MessageHashing.sol";

/**
 * @title Contract to manage pre-existing cross-chain messaging-claiming function.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
abstract contract ClaimMessageV1 is IClaimMessageV1, L1MessageServiceBase {
  /**
   * @notice Claims and delivers a cross-chain message.
   * @dev _feeRecipient can be set to address(0) to receive as msg.sender.
   * @dev The original message sender address is temporarily set in transient storage,
   * while claiming. This address is used in sender().
   * @param _from The address of the original sender.
   * @param _to The address the message is intended for.
   * @param _fee The fee being paid for the message delivery.
   * @param _value The value to be transferred to the destination address.
   * @param _feeRecipient The recipient for the fee.
   * @param _calldata The calldata to pass to the recipient.
   * @param _nonce The unique auto generated nonce used when sending the message.
   */
  function claimMessage(
    address _from,
    address _to,
    uint256 _fee,
    uint256 _value,
    address payable _feeRecipient,
    bytes calldata _calldata,
    uint256 _nonce
  ) public virtual nonReentrant distributeFees(_fee, _to, _calldata, _feeRecipient) {
    _requireTypeAndGeneralNotPaused(PauseType.L2_L1);

    /// @dev This is placed earlier to fix the stack issue by using these two earlier on.
    TRANSIENT_MESSAGE_SENDER = _from;

    bytes32 messageHash = MessageHashing._hashMessage(_from, _to, _fee, _value, _nonce, _calldata);

    // @dev Status check and revert is in the message manager.
    _updateL2L1MessageStatusToClaimed(messageHash);

    _addUsedAmount(_fee + _value);

    (bool callSuccess, bytes memory returnData) = _to.call{ value: _value }(_calldata);
    if (!callSuccess) {
      if (returnData.length > 0) {
        assembly {
          let data_size := mload(returnData)
          revert(add(32, returnData), data_size)
        }
      } else {
        revert MessageSendingFailed(_to);
      }
    }

    TRANSIENT_MESSAGE_SENDER = DEFAULT_MESSAGE_SENDER_TRANSIENT_VALUE;

    emit MessageClaimed(messageHash);
  }
}
