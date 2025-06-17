// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.30;

import { Initializable } from "@openzeppelin/contracts-upgradeable/proxy/utils/Initializable.sol";
import { RateLimiter } from "../../../security/limiting/RateLimiter.sol";

contract TestRateLimiter is Initializable, RateLimiter {
  // we need eth to test the limits with
  function initialize(uint256 _periodInSeconds, uint256 _limitInWei) public initializer {
    __AccessControl_init();

    __RateLimiter_init(_periodInSeconds, _limitInWei);
    _grantRole(DEFAULT_ADMIN_ROLE, msg.sender);
  }

  // @dev this is needed to get at the internal function
  function withdrawSomeAmount(uint256 _amount) external {
    _addUsedAmount(_amount);
  }

  function tryInitialize(uint256 _periodInSeconds, uint256 _limitInWei) external {
    __RateLimiter_init(_periodInSeconds, _limitInWei);
  }
}
