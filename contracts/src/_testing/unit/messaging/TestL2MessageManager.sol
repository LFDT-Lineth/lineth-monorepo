// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.33;

import { Initializable } from "@openzeppelin/contracts-upgradeable/proxy/utils/Initializable.sol";
import { L2MessageManager } from "../../../messaging/l2/L2MessageManager.sol";
import { IGenericErrors } from "../../../interfaces/IGenericErrors.sol";
import { TestSetPauseTypeRoles } from "../security/TestSetPauseTypeRoles.sol";

contract TestL2MessageManager is Initializable, L2MessageManager, IGenericErrors, TestSetPauseTypeRoles {
  /// @custom:oz-upgrades-unsafe-allow constructor
  constructor() {
    _disableInitializers();
  }

  function initialize(
    address _pauserManager,
    address _l1l2MessageSetter,
    PauseTypeRole[] calldata _pauseTypeRoles,
    PauseTypeRole[] calldata _unpauseTypeRoles
  ) public initializer {
    if (_pauserManager == address(0)) {
      revert ZeroAddressNotAllowed();
    }

    if (_l1l2MessageSetter == address(0)) {
      revert ZeroAddressNotAllowed();
    }

    __ERC165_init();
    __Context_init();
    __AccessControl_init();
    __PauseManager_init(_pauseTypeRoles, _unpauseTypeRoles);

    _grantRole(DEFAULT_ADMIN_ROLE, msg.sender);
    _grantRole(PAUSE_ALL_ROLE, _pauserManager);
    _grantRole(UNPAUSE_ALL_ROLE, _pauserManager);
    _grantRole(L1_L2_MESSAGE_SETTER_ROLE, _l1l2MessageSetter);
  }

  function updateL1L2MessageStatusToClaimed(bytes32 _messageHash) external {
    _updateL1L2MessageStatusToClaimed(_messageHash);
  }

  function setLastAnchoredL1MessageNumber(uint256 _messageNumber) external {
    lastAnchoredL1MessageNumber = _messageNumber;
  }
}
