// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.30;

import { AccessControl } from "@openzeppelin/contracts/access/AccessControl.sol";
import { ITokenMinter } from "./interfaces/ITokenMinter.sol";
import { ITokenMintingRateLimiter } from "./interfaces/ITokenMintingRateLimiter.sol";

/**
 * @title Token minting rate limiter.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 * @dev State variables are public for ease of consumption.
 */
contract TokenMintingRateLimiter is ITokenMinter, ITokenMintingRateLimiter, AccessControl {
  bytes32 public constant RATE_LIMIT_SETTER_ROLE = keccak256("RATE_LIMIT_SETTER_ROLE");
  bytes32 public constant MINTER_ROLE = keccak256("MINTER_ROLE");

  // @notice How much time before limit resets.
  uint256 public mintingPeriodInSeconds;

  // @notice Max minted tokens in the time period.
  uint256 public mintingLimit;

  // @notice The address of the token being minted.
  ITokenMinter public tokenAddress;

  // @notice The time at which the current period ends at.
  uint256 public currentPeriodEnd;

  // @notice Amounts already withdrawn this period.
  uint256 public mintedAmountInPeriod;

  /**
   * @notice Constructs the smart contract.
   * @param _tokenAddress The address of the token being minted.
   * @param _mintingPeriodInSeconds The minting period in seconds.
   * @param _mintingLimit The minting limit.
   * @param _defaultAdmin The default admin address.
   * @param _defaultMinter The default address allowed to mint.
   */
  constructor(
    address _tokenAddress,
    uint256 _mintingPeriodInSeconds,
    uint256 _mintingLimit,
    address _defaultAdmin,
    address _defaultMinter
  ) {
    if (_tokenAddress == address(0)) {
      revert ZeroAddressNotAllowed();
    }

    if (_defaultAdmin == address(0)) {
      revert ZeroAddressNotAllowed();
    }

    if (_defaultMinter == address(0)) {
      revert ZeroAddressNotAllowed();
    }

    if (_mintingPeriodInSeconds == 0) {
      revert PeriodIsZero();
    }

    if (_mintingLimit == 0) {
      revert LimitIsZero();
    }

    tokenAddress = ITokenMinter(_tokenAddress);
    mintingPeriodInSeconds = _mintingPeriodInSeconds;
    mintingLimit = _mintingLimit;

    _grantRole(DEFAULT_ADMIN_ROLE, _defaultAdmin);
    _grantRole(RATE_LIMIT_SETTER_ROLE, _defaultAdmin);
    _grantRole(MINTER_ROLE, _defaultMinter);

    uint256 mintingPeriodEnd = block.timestamp + _mintingPeriodInSeconds;
    currentPeriodEnd = mintingPeriodEnd;

    emit RateLimitInitialized(_mintingPeriodInSeconds, _mintingLimit, mintingPeriodEnd);
  }

  /**
   * @notice Mints a single token amount for a single recipient.
   * @param _to The address receiving the token amount.
   * @param _amount The amount of token to receive.
   * @dev Only the MINTER_ROLE can mint these tokens
   */
  function mint(address _to, uint256 _amount) external onlyRole(MINTER_ROLE) {
    _addUsedAmount(_amount);

    tokenAddress.mint(_to, _amount);
  }

  /**
   * @notice Mints a single token amount for a multiple recipients.
   * @param _to The addresses receiving the token amount.
   * @param _amount The amount of token to receive.
   * @dev Only the MINTER_ROLE can mint these tokens
   * @dev Always do an eth_call simular
   */
  function batchMint(address[] calldata _to, uint256 _amount) external onlyRole(MINTER_ROLE) {
    _addUsedAmount(_to.length * _amount);

    tokenAddress.batchMint(_to, _amount);
  }

  /**
   * @notice Mints a 1:1 amounts for multiple recipients.
   * @param _to The addresses receiving the token amount.
   * @param _amounts The amounts of token to receive.
   * @dev Only the MINTER_ROLE can mint these tokens
   */
  function batchMintMultiple(address[] calldata _to, uint256[] calldata _amounts) external onlyRole(MINTER_ROLE) {
    uint256 addressLength = _to.length;

    if (addressLength != _amounts.length) {
      revert ArrayLengthsDoNotMatch();
    }

    uint256 mintAmount;

    for (uint256 i; i < addressLength; ) {
      mintAmount += _amounts[i];
      unchecked {
        ++i;
      }
    }

    _addUsedAmount(mintAmount);

    tokenAddress.batchMintMultiple(_to, _amounts);
  }

  /**
   * @notice Increments the amount used in the period.
   * @dev The amount determining logic is external to this (e.g. fees are included when calling here).
   * @dev Reverts if the limit is breached.
   * @param _usedAmount The amount used to be added.
   */
  function _addUsedAmount(uint256 _usedAmount) internal {
    uint256 currentPeriodAmountTemp;

    if (currentPeriodEnd < block.timestamp) {
      currentPeriodEnd = block.timestamp + mintingPeriodInSeconds;
      currentPeriodAmountTemp = _usedAmount;
    } else {
      currentPeriodAmountTemp = mintedAmountInPeriod + _usedAmount;
    }

    if (currentPeriodAmountTemp > mintingLimit) {
      revert RateLimitExceeded();
    }

    mintedAmountInPeriod = currentPeriodAmountTemp;
  }

  /**
   * @notice Resets the rate limit amount.
   * @dev If the used amount is higher, it is set to the limit to avoid confusion/issues.
   * @dev Only the RATE_LIMIT_SETTER_ROLE is allowed to execute this function.
   * @dev Emits the LimitAmountChanged event.
   * @dev usedLimitAmountToSet will use the default value of zero if period has expired
   * @param _amount The amount to reset the limit to.
   */
  function resetRateLimitAmount(uint256 _amount) external onlyRole(RATE_LIMIT_SETTER_ROLE) {
    uint256 usedLimitAmountToSet;
    bool amountUsedLoweredToLimit;
    bool usedAmountResetToZero;

    if (currentPeriodEnd < block.timestamp) {
      currentPeriodEnd = block.timestamp + mintingPeriodInSeconds;
      usedAmountResetToZero = true;
    } else {
      if (_amount < mintedAmountInPeriod) {
        usedLimitAmountToSet = _amount;
        amountUsedLoweredToLimit = true;
      }
    }

    mintingLimit = _amount;

    if (usedAmountResetToZero || amountUsedLoweredToLimit) {
      mintedAmountInPeriod = usedLimitAmountToSet;
    }

    emit LimitAmountChanged(_msgSender(), _amount, amountUsedLoweredToLimit, usedAmountResetToZero);
  }
}
