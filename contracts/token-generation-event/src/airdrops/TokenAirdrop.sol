// SPDX-License-Identifier: Apache-2.0 OR MIT
pragma solidity 0.8.30;

import { Ownable, Ownable2Step } from "@openzeppelin/contracts/access/Ownable2Step.sol";
import { IERC20 } from "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import { SafeERC20 } from "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";

/**
 * @title Contract to manage a token airdrop.
 * @author Consensys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
contract TokenAirdrop is Ownable2Step {
  using SafeERC20 for IERC20;

  /// @notice Emitted when a user claims tokens.
  /// @param user The user address.
  /// @param amount The amount of tokens claimed.
  event Claimed(address indexed user, uint256 amount);

  /**
   * @notice Emitted on construction and airdrop contract configured.
   * @param token Token address used in the airdrop.
   * @param primaryFactor The primary factor address.
   * @param primaryMultiplier The primary conditional multiplier address.
   * @param secondaryFactor The secondary factor address.
   * @param claimEndTimestamp The end of the claim period.
   */
  event AirdropConfigured(
    address token,
    address primaryFactor,
    address primaryMultiplier,
    address secondaryFactor,
    uint256 claimEndTimestamp
  );

  /// @notice Emitted when the owner withdraws tokens.
  /// @param owner The owner address.
  /// @param amount The amount of tokens withdrawn.
  event TokenBalanceWithdrawn(address owner, uint256 amount);

  /// @dev Thrown when an address is the zero address.
  error ZeroAddressNotAllowed();

  /// @dev Thrown when the claim end is in the past.
  error ClaimEndTooShort();

  /// @dev Thrown when all addresses and multiplier values are zero address or zero.
  error AllCalculationFactorsZero();

  /// @dev Thrown when an address has already had their tokens claimed.
  error AlreadyClaimed();

  /// @dev Thrown when trying to claim after the claim period.
  error ClaimFinished();

  /// @dev Thrown when the owner tries to withdraw before the end of the claim period.
  error ClaimNotFinished();

  /// @dev Thrown when a user is attempting to claim zero tokens.
  error ClaimAmountIsZero();

  /// @dev Thrown when an owner attempts to renounce ownership.
  error RenouncingOwnershipDisabled();

  /// @dev Denominator used for division when multiplying out the token allocation.
  uint256 public constant DENOMINATOR = 1e9;

  /// @notice The token contract.
  IERC20 public immutable TOKEN;

  /// @notice The timestamp when the claim period ends.
  uint256 public immutable CLAIM_END;

  /// @notice The primary contract affecting airdrop calculation values by balance.
  IERC20 public immutable PRIMARY_FACTOR_ADDRESS;

  /// @notice The address of the contract providing an additional multiplier to the primary balance which requires
  /// DENOMINATOR division.
  IERC20 public immutable PRIMARY_CONDITIONAL_MULTIPLIER_ADDRESS;

  /// @notice The secondary contract affecting airdrop calculation values by balance.
  IERC20 public immutable SECONDARY_FACTOR_ADDRESS;

  /// @notice Mapping of claimed status.
  mapping(address user => bool claimed) public hasClaimed;

  /**
   * @notice Defines all needed immutable variables.
   * @dev The primary and secondary factor (or their multipliers) can be address(0) or 0 but not all of them.
   * @param _token The token to transfer.
   * @param _ownerAddress The airdrop contract owner address.
   * @param _claimEnd The timestamp when the claim period ends.
   * @param _primaryFactorAddress The primary address.
   * @param _primaryConditionalMultiplierAddress The primary multiplier address.
   * @param _secondaryFactorAddress The secondary address.
   */
  constructor(
    address _token,
    address _ownerAddress,
    uint256 _claimEnd,
    address _primaryFactorAddress,
    address _primaryConditionalMultiplierAddress,
    address _secondaryFactorAddress
  ) Ownable(_ownerAddress) {
    require(_token != address(0), ZeroAddressNotAllowed());
    require(block.timestamp < _claimEnd, ClaimEndTooShort());

    require(
      _primaryFactorAddress != address(0) ||
        _primaryConditionalMultiplierAddress != address(0) ||
        _secondaryFactorAddress != address(0),
      AllCalculationFactorsZero()
    );

    require(
      _primaryFactorAddress != address(0) || _primaryConditionalMultiplierAddress == address(0),
      ZeroAddressNotAllowed()
    );

    require(
      _primaryFactorAddress == address(0) || _primaryConditionalMultiplierAddress != address(0),
      ZeroAddressNotAllowed()
    );

    TOKEN = IERC20(_token);
    PRIMARY_FACTOR_ADDRESS = IERC20(_primaryFactorAddress);
    PRIMARY_CONDITIONAL_MULTIPLIER_ADDRESS = IERC20(_primaryConditionalMultiplierAddress);
    SECONDARY_FACTOR_ADDRESS = IERC20(_secondaryFactorAddress);
    CLAIM_END = _claimEnd;

    emit AirdropConfigured(
      _token,
      _primaryFactorAddress,
      _primaryConditionalMultiplierAddress,
      _secondaryFactorAddress,
      _claimEnd
    );
  }

  /**
   * @notice Calculates the claim value for an account for with either primary or secondary balances, or both.
   * @dev NB: We multiply out and then divide by the DENOMINATOR `PER` multiplier (e.g. xMultiplier of
   * 1200000000/DENOMINATOR yields a 1.2x ).
   * @param _account Account being calculated for.
   * @return tokenAllocation Formula=((primaryBalance * primaryConditionalMultiplier) / DENOMINATOR) + secondaryBalance.
   */
  function calculateAllocation(address _account) public view returns (uint256 tokenAllocation) {
    if (address(PRIMARY_FACTOR_ADDRESS) != address(0)) {
      tokenAllocation =
        (PRIMARY_FACTOR_ADDRESS.balanceOf(_account) * PRIMARY_CONDITIONAL_MULTIPLIER_ADDRESS.balanceOf(_account)) /
        DENOMINATOR;
    }

    if (address(SECONDARY_FACTOR_ADDRESS) != address(0)) {
      tokenAllocation += SECONDARY_FACTOR_ADDRESS.balanceOf(_account);
    }
  }

  /**
   * @notice Claims tokens for the caller account while the claiming is active.
   * @dev Claiming sets claim status pre-transferring avoiding reentry, and all multiplier tokens are soulbound avoiding
   * transfer manipulation.
   */
  function claim() external {
    require(block.timestamp < CLAIM_END, ClaimFinished());
    require(!hasClaimed[msg.sender], AlreadyClaimed());

    uint256 tokenAmount = calculateAllocation(msg.sender);

    require(tokenAmount != 0, ClaimAmountIsZero());

    hasClaimed[msg.sender] = true;
    emit Claimed(msg.sender, tokenAmount);

    TOKEN.safeTransfer(msg.sender, tokenAmount);
  }

  /**
   * @notice Owner withdraws unclaimed tokens from the contract post claim end.
   */
  function withdraw() external onlyOwner {
    require(CLAIM_END <= block.timestamp, ClaimNotFinished());

    uint256 balance = TOKEN.balanceOf(address(this));

    emit TokenBalanceWithdrawn(msg.sender, balance);
    TOKEN.safeTransfer(msg.sender, balance);
  }

  /**
   * @notice Overrides renounceOwnership preventing renouncing.
   * @dev While this will always revert, the onlyOwner is left in for consistency.
   */
  function renounceOwnership() public view override onlyOwner {
    revert RenouncingOwnershipDisabled();
  }
}
