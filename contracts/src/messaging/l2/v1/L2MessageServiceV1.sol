// SPDX-License-Identifier: AGPL-3.0
pragma solidity 0.8.19;

import { ReentrancyGuardUpgradeable } from "@openzeppelin/contracts-upgradeable/security/ReentrancyGuardUpgradeable.sol";
import { IMessageService } from "../../interfaces/IMessageService.sol";
import { IL2MessageServiceV1 } from "./interfaces/IL2MessageServiceV1.sol";
import { IGenericErrors } from "../../../interfaces/IGenericErrors.sol";
import { RateLimiter } from "../../../security/limiting/RateLimiter.sol";
import { L2MessageManagerV1 } from "./L2MessageManagerV1.sol";
import { MessageHashing } from "../../libraries/MessageHashing.sol";

/**
 * @title Contract to manage cross-chain messaging on L2.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
abstract contract L2MessageServiceV1 is
  RateLimiter,
  L2MessageManagerV1,
  ReentrancyGuardUpgradeable,
  IMessageService,
  IL2MessageServiceV1,
  IGenericErrors
{
  using MessageHashing for *;

  /**
   * @dev Keep 50 free storage slots for future implementation updates to avoid storage collision.
   * NB: Take note that this is at the beginning of the file where other storage gaps,
   * are at the end of files. Be careful with how storage is adjusted on upgrades.
   */
  uint256[50] private __gap_L2MessageService;

  /// @notice The role required to set the minimum DDOS fee.
  bytes32 public constant MINIMUM_FEE_SETTER_ROLE = keccak256("MINIMUM_FEE_SETTER_ROLE");

  /// @dev The temporary message sender set when claiming a message.
  address internal _messageSender;

  // @notice initialize to save user cost with existing slot.
  uint256 public nextMessageNumber;

  // @notice initialize minimumFeeInWei variable.
  uint256 public minimumFeeInWei;

  // @dev adding these should not affect storage as they are constants and are stored in bytecode.
  uint256 internal constant REFUND_OVERHEAD_IN_GAS = 44596;

  /// @dev The default message sender address reset after claiming a message.
  address internal constant DEFAULT_SENDER_ADDRESS = address(123456789);

  /// @dev Total contract storage is 53 slots including the gap above. NB: Above!

  /// @custom:oz-upgrades-unsafe-allow constructor
  constructor() {
    _disableInitializers();
  }

  /**
   * @notice Adds a message for sending cross-chain and emits a relevant event.
   * @dev The message number is preset and only incremented at the end if successful for the next caller.
   * @param _to The address the message is intended for.
   * @param _fee The fee being paid for the message delivery.
   * @param _calldata The calldata to pass to the recipient.
   */
  function sendMessage(address _to, uint256 _fee, bytes calldata _calldata) external payable {
    _requireTypeAndGeneralNotPaused(PauseType.L2_L1);

    if (_to == address(0)) {
      revert ZeroAddressNotAllowed();
    }

    if (_fee > msg.value) {
      revert ValueSentTooLow();
    }

    uint256 coinbaseFee = minimumFeeInWei;

    if (_fee < coinbaseFee) {
      revert FeeTooLow();
    }

    uint256 postmanFee;
    uint256 valueSent;

    postmanFee = _fee - coinbaseFee;
    valueSent = msg.value - _fee;

    uint256 messageNumber = nextMessageNumber++;

    /// @dev Rate limit and revert is in the rate limiter.
    _addUsedAmount(valueSent + postmanFee);

    bytes32 messageHash = MessageHashing._hashMessage(msg.sender, _to, postmanFee, valueSent, messageNumber, _calldata);

    emit MessageSent(msg.sender, _to, postmanFee, valueSent, messageNumber, _calldata, messageHash);

    (bool success, ) = block.coinbase.call{ value: coinbaseFee }("");
    if (!success) {
      revert FeePaymentFailed(block.coinbase);
    }
  }

  /**
   * @notice Claims and delivers a cross-chain message.
   * @dev _feeRecipient Can be set to address(0) to receive as msg.sender.
   * @dev messageSender Is set temporarily when claiming and reset post.
   * @param _from The address of the original sender.
   * @param _to The address the message is intended for.
   * @param _fee The fee being paid for the message delivery.
   * @param _value The value to be transferred to the destination address.
   * @param _feeRecipient The recipient for the fee.
   * @param _calldata The calldata to pass to the recipient.
   * @param _nonce The unique auto generated message number used when sending the message.
   */
  function claimMessage(
    address _from,
    address _to,
    uint256 _fee,
    uint256 _value,
    address payable _feeRecipient,
    bytes calldata _calldata,
    uint256 _nonce
  ) external nonReentrant distributeFees(_fee, _to, _calldata, _feeRecipient) {
    _requireTypeAndGeneralNotPaused(PauseType.L1_L2);

    bytes32 messageHash = MessageHashing._hashMessage(_from, _to, _fee, _value, _nonce, _calldata);

    /// @dev Status check and revert is in the message manager.
    _updateL1L2MessageStatusToClaimed(messageHash);

    _messageSender = _from;

    (bool callSuccess, bytes memory returnData) = _to.call{ value: _value }(_calldata);
    if (!callSuccess) {
      if (returnData.length > 0) {
        assembly {
          let data_size := mload(returnData)
          revert(add(0x20, returnData), data_size)
        }
      } else {
        revert MessageSendingFailed(_to);
      }
    }

    _messageSender = DEFAULT_SENDER_ADDRESS;
    emit MessageClaimed(messageHash);
  }

  /**
   * @notice The Fee Manager sets a minimum fee to address DOS protection.
   * @dev MINIMUM_FEE_SETTER_ROLE is required to set the minimum fee.
   * @param _feeInWei New minimum fee in Wei.
   */
  function setMinimumFee(uint256 _feeInWei) external onlyRole(MINIMUM_FEE_SETTER_ROLE) {
    uint256 previousMinimumFee = minimumFeeInWei;
    minimumFeeInWei = _feeInWei;

    emit MinimumFeeChanged(previousMinimumFee, _feeInWei, msg.sender);
  }

  /**
   * @dev The _messageSender address is set temporarily when claiming.
   * @return originalSender The original sender stored temporarily at the _messageSender address in storage.
   */
  function sender() external view returns (address originalSender) {
    originalSender = _messageSender;
  }

  /**
   * @notice The unspent fee is refunded if applicable.
   * @param _feeInWei The fee paid for delivery in Wei.
   * @param _to The recipient of the message and gas refund.
   * @param _calldata The calldata of the message.
   */
  modifier distributeFees(
    uint256 _feeInWei,
    address _to,
    bytes calldata _calldata,
    address _feeRecipient
  ) {
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
