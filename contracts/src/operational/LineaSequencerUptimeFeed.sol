// SPDX-License-Identifier: AGPL-3.0
pragma solidity 0.8.33;

import { BaseSequencerUptimeFeed } from "@chainlink/contracts/src/v0.8/l2ep/base/BaseSequencerUptimeFeed.sol";
import { AccessControl } from "@openzeppelin/contracts/access/AccessControl.sol";
import { ILineaSequencerUptimeFeed } from "./interfaces/ILineaSequencerUptimeFeed.sol";

/**
 * @title LineaSequencerUptimeFeed - L2 sequencer uptime status aggregator.
 * @notice L2 contract that receives status updates,
 *  and records a new answer if the status changed.
 */
contract LineaSequencerUptimeFeed is BaseSequencerUptimeFeed, AccessControl, ILineaSequencerUptimeFeed {
  string public constant override typeAndVersion = "LineaSequencerUptimeFeed 1.0.0";

  bytes32 public constant FEED_UPDATER_ROLE = keccak256("FEED_UPDATER_ROLE");

  /**
   * @param _initialStatus The initial status of the feed.
   * @param _admin The address of the admin that can manage the feed.
   * @param _feedUpdater The address of the feed updater that can update the status.
   * @dev NB: The l1Sender parameter in the BaseSequencerUptimeFeed constructor is set as address(0) because it is not used.
   * AccessControl is used instead to manage permissions.
   */
  constructor(
    bool _initialStatus,
    address _admin,
    address _feedUpdater
  ) BaseSequencerUptimeFeed(address(0), _initialStatus) {
    require(_admin != address(0), ZeroAddressNotAllowed());
    require(_feedUpdater != address(0), ZeroAddressNotAllowed());

    _grantRole(DEFAULT_ADMIN_ROLE, _admin);
    _grantRole(FEED_UPDATER_ROLE, _feedUpdater);
  }

  /**
   * @notice Internal function to check if the sender is allowed to update the status.
   * @dev NB: The parameter is not used here because the function relies on msg.sender instead of l1Sender.
   * @dev NB: The l1Sender storage variable from the BaseSequencerUptimeFeed contract is not used.
   * AccessControl is used instead to manage permissions.
   */
  function _validateSender(address) internal view override {
    require(hasRole(FEED_UPDATER_ROLE, msg.sender), InvalidSender());
  }
}
