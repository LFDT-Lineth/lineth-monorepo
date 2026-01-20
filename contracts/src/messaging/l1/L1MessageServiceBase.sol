// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.33;

import { LineaRollupPauseManager } from "../../security/pausing/LineaRollupPauseManager.sol";
import { RateLimiter } from "../../security/limiting/RateLimiter.sol";
import { L1MessageManagerV1 } from "./v1/L1MessageManagerV1.sol";
import { TransientStorageReentrancyGuardUpgradeable } from "../../security/reentrancy/TransientStorageReentrancyGuardUpgradeable.sol";
import { IMessageService } from "../interfaces/IMessageService.sol";
import { MessageHashing } from "../libraries/MessageHashing.sol";

/**
 * @title Contract to manage cross-chain messaging on L1.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
abstract contract L1MessageServiceBase is
  RateLimiter,
  L1MessageManagerV1,
  TransientStorageReentrancyGuardUpgradeable,
  LineaRollupPauseManager,
  IMessageService
{
  using MessageHashing for *;

  address transient TRANSIENT_MESSAGE_SENDER;

  // @dev This is initialised to save user cost with existing slot.
  uint256 public nextMessageNumber;

  /// @dev DEPRECATED in favor of new transient storage with `MESSAGE_SENDER_TRANSIENT_KEY` key.
  address private _messageSender_DEPRECATED;

  /// @dev Total contract storage is 52 slots including the gap below.
  /// @dev Keep 50 free storage slots for future implementation updates to avoid storage collision.
  uint256[50] private __gap;

  /// @dev adding these should not affect storage as they are constants and are stored in bytecode.
  uint256 internal constant REFUND_OVERHEAD_IN_GAS = 48252;

  /// @notice The default value for the message sender reset to post claiming using the MESSAGE_SENDER_TRANSIENT_KEY.
  address internal constant DEFAULT_MESSAGE_SENDER_TRANSIENT_VALUE = address(0);

  /**
   * @notice The unspent fee is refunded if applicable.
   * @param _feeInWei The fee paid for delivery in Wei.
   * @param _to The recipient of the message and gas refund.
   * @param _calldata The calldata of the message.
   */
  modifier distributeFees(uint256 _feeInWei, address _to, bytes calldata _calldata, address _feeRecipient) {
    //pre-execution
    uint256 startingGas = gasleft();
    _;
    //post-execution

    // we have a fee
    if (_feeInWei > 0) {
      // default postman fee
      uint256 deliveryFee = _feeInWei;

      // do we have empty calldata?
      if (_calldata.length == 0) {
        bool isDestinationEOA;

        assembly {
          isDestinationEOA := iszero(extcodesize(_to))
        }

        // are we calling an EOA
        if (isDestinationEOA) {
          // initial + cost to call and refund minus gasleft
          deliveryFee = (startingGas + REFUND_OVERHEAD_IN_GAS - gasleft()) * tx.gasprice;

          if (_feeInWei > deliveryFee) {
            payable(_to).send(_feeInWei - deliveryFee);
          } else {
            deliveryFee = _feeInWei;
          }
        }
      }

      address feeReceiver = _feeRecipient == address(0) ? msg.sender : _feeRecipient;

      bool callSuccess = payable(feeReceiver).send(deliveryFee);
      if (!callSuccess) {
        revert FeePaymentFailed(feeReceiver);
      }
    }
  }
}
