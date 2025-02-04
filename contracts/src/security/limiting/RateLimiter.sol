// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.19;

import { AccessControlUpgradeable } from "@openzeppelin/contracts-upgradeable/access/AccessControlUpgradeable.sol";
import { IRateLimiter } from "./interfaces/IRateLimiter.sol";

/**
 * @title Rate Limiter by period and amount using the block timestamp.
 * @author ConsenSys Software Inc.
 * @notice You can use this control numeric limits over a period using timestamp.
 * @custom:security-contact security-report@linea.build
 */
abstract contract RateLimiter is IRateLimiter, AccessControlUpgradeable {
  bytes32 public constant RATE_LIMIT_SETTER_ROLE = keccak256("RATE_LIMIT_SETTER_ROLE");
  bytes32 public constant USED_RATE_LIMIT_RESETTER_ROLE = keccak256("USED_RATE_LIMIT_RESETTER_ROLE");

  uint256 public periodInSeconds; // how much time before limit resets.
  uint256 public limitInWei; // max ether to withdraw per period.

  /// @dev Public for ease of consumption.
  /// @notice The time at which the current period ends at.
  uint256 public currentPeriodEnd;

  /// @dev Public for ease of consumption.
  /// @notice Amounts already withdrawn this period.
  uint256 public currentPeriodAmountInWei;

  /// @dev Total contract storage is 14 slots with the gap below.
  /// @dev Keep 10 free storage slots for future implementation updates to avoid storage collision.
  uint256[10] private __gap;

  /**
   * @notice Initialises the limits and period for the rate limiter.
   * @param _periodInSeconds The length of the period in seconds.
   * @param _limitInWei The limit allowed in the period in Wei.
   */
  function __RateLimiter_init(uint256 _periodInSeconds, uint256 _limitInWei) internal onlyInitializing {
    if (_periodInSeconds == 0) {
      revert PeriodIsZero();
    }

    if (_limitInWei == 0) {
      revert LimitIsZero();
    }

    periodInSeconds = _periodInSeconds;
    limitInWei = _limitInWei;
    currentPeriodEnd = block.timestamp + _periodInSeconds;

    emit RateLimitInitialized(periodInSeconds, limitInWei, currentPeriodEnd);
  }

  /**
   * @notice Increments the amount used in the period.
   * @dev The amount determining logic is external to this (e.g. fees are included when calling here).
   * @dev Ignores the calculation if _usedAmount is zero.
   * @dev Reverts if the limit is breached.
   * @param _usedAmount The amount used to be added.
   */
  function _addUsedAmount(uint256 _usedAmount) internal {
    if (_usedAmount != 0) {
      if (currentPeriodEnd < block.timestamp) {
        currentPeriodEnd = block.timestamp + periodInSeconds;
      } else {
        _usedAmount += currentPeriodAmountInWei;
      }

      if (_usedAmount > limitInWei) {
        revert RateLimitExceeded();
      }

      currentPeriodAmountInWei = _usedAmount;
    }
  }

  /**
   * @notice Resets the rate limit amount.
   * @dev If the used amount is higher, it is set to the limit to avoid confusion/issues.
   * @dev Only the RATE_LIMIT_SETTER_ROLE is allowed to execute this function.
   * @dev Emits the LimitAmountChanged event.
   * @dev usedLimitAmountToSet will use the default value of zero if period has expired.
   * @param _amount The amount to reset the limit to.
   */
  function resetRateLimitAmount(uint256 _amount) external onlyRole(RATE_LIMIT_SETTER_ROLE) {
    uint256 usedLimitAmountToSet;
    bool amountUsedLoweredToLimit;
    bool usedAmountResetToZero;

    if (currentPeriodEnd < block.timestamp) {
      currentPeriodEnd = block.timestamp + periodInSeconds;
      usedAmountResetToZero = true;
    } else {
      if (_amount < currentPeriodAmountInWei) {
        usedLimitAmountToSet = _amount;
        amountUsedLoweredToLimit = true;
      }
    }

    limitInWei = _amount;

    if (usedAmountResetToZero || amountUsedLoweredToLimit) {
      currentPeriodAmountInWei = usedLimitAmountToSet;
    }

    emit LimitAmountChanged(_msgSender(), _amount, amountUsedLoweredToLimit, usedAmountResetToZero);
  }

  /**
   * @notice Resets the amount used to zero.
   * @dev Only the USED_RATE_LIMIT_RESETTER_ROLE is allowed to execute this function.
   * @dev Emits the AmountUsedInPeriodReset event.
   */
  function resetAmountUsedInPeriod() external onlyRole(USED_RATE_LIMIT_RESETTER_ROLE) {
    currentPeriodAmountInWei = 0;

    emit AmountUsedInPeriodReset(_msgSender());
  }
}
