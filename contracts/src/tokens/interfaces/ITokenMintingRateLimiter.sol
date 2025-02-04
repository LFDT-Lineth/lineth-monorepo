// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.19;

/**
 * @title Token Minting Rate Limiter Interface.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
interface ITokenMintingRateLimiter {
  /**
   * @dev Thrown when a parameter is the zero address.
   */
  error ZeroAddressNotAllowed();

  /**
   * @dev Thrown when an amount breaches the limit in the period.
   */
  error RateLimitExceeded();

  /**
   * @dev Thrown when the period is initialised to zero.
   */
  error PeriodIsZero();

  /**
   * @dev Thrown when the limit is initialised to zero.
   */
  error LimitIsZero();

  /**
   * @dev Thrown when array lengths are mismatched.
   */
  error ArrayLengthsDoNotMatch();

  /**
   * @dev Emitted when the Rate Limit is initialized.
   */
  event RateLimitInitialized(uint256 mintingPeriodInSeconds, uint256 mintingLimit, uint256 currentPeriodEnd);

  /**
   * @dev Emitted when the limit is changed.
   * @dev If the current used amount is higher than the new limit, the used amount is lowered to the limit.
   */
  event LimitAmountChanged(
    address indexed amountChangeBy,
    uint256 amount,
    bool amountUsedLoweredToLimit,
    bool usedAmountResetToZero
  );

  /**
   * @notice Resets the rate limit amount to the amount specified.
   * @param _amount sets the new limit amount.
   * @dev Requires RATE_LIMIT_SETTER_ROLE.
   */
  function resetRateLimitAmount(uint256 _amount) external;
}
