// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.19;

import { Initializable } from "@openzeppelin/contracts-upgradeable/proxy/utils/Initializable.sol";
import { IMessageService } from "./interfaces/IMessageService.sol";
import { IGenericErrors } from "../interfaces/IGenericErrors.sol";

/**
 * @title Base contract to manage cross-chain messaging.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
abstract contract MessageServiceBase is Initializable, IGenericErrors {
  /// @notice The message service address on the current chain.
  IMessageService public messageService;

  /// @notice The token bridge on the alternate/remote chain.
  address public remoteSender;

  /// @dev Total contract storage is 12 slots with the gap below.
  /// @dev Keep 10 free storage slots for future implementation updates to avoid storage collision.
  uint256[10] private __base_gap;

  /**
   * @dev Event emitted when the remote sender is set.
   * @param remoteSender The address of the new remote sender.
   * @param setter The address of the account that set the remote sender.
   */
  event RemoteSenderSet(address indexed remoteSender, address indexed setter);

  /**
   * @dev Thrown when the caller address is not the message service address
   */
  error CallerIsNotMessageService();

  /**
   * @dev Thrown when remote sender address is not authorized.
   */
  error SenderNotAuthorized();

  /**
   * @dev Modifier to make sure the caller is the known message service.
   *
   * Requirements:
   *
   * - The msg.sender must be the message service.
   */
  modifier onlyMessagingService() {
    if (msg.sender != address(messageService)) {
      revert CallerIsNotMessageService();
    }
    _;
  }

  /**
   * @dev Modifier to make sure the original sender is allowed.
   *
   * Requirements:
   *
   * - The original message sender via the message service must be a known sender.
   */
  modifier onlyAuthorizedRemoteSender() {
    if (messageService.sender() != remoteSender) {
      revert SenderNotAuthorized();
    }
    _;
  }

  /**
   * @notice Initializes the message service
   * @dev Must be initialized in the initialize function of the main contract or constructor.
   * @param _messageService The message service address, cannot be empty.
   */
  function __MessageServiceBase_init(address _messageService) internal onlyInitializing {
    if (_messageService == address(0)) {
      revert ZeroAddressNotAllowed();
    }

    messageService = IMessageService(_messageService);
  }

  /**
   * @notice Sets the remote sender
   * @dev This function sets the remote sender address and emits the RemoteSenderSet event.
   * @param _remoteSender The authorized remote sender address, cannot be empty.
   */
  function _setRemoteSender(address _remoteSender) internal {
    if (_remoteSender == address(0)) {
      revert ZeroAddressNotAllowed();
    }

    remoteSender = _remoteSender;
    emit RemoteSenderSet(_remoteSender, msg.sender);
  }
}
