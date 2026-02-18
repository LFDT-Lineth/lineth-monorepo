// SPDX-License-Identifier: Apache-2.0 OR MIT
pragma solidity 0.8.30;

import "forge-std/Test.sol";
import { TokenAirdrop } from "../../src/airdrops/TokenAirdrop.sol";
import { Ownable, Ownable2Step } from "@openzeppelin/contracts/access/Ownable2Step.sol";

/// @notice A minimal ERC20 mock for testing purposes.
contract MockERC20 {
  string public name;
  string public symbol;
  uint8 public decimals;
  uint256 public totalSupply;
  mapping(address => uint256) public balanceOf;

  event Transfer(address indexed from, address indexed to, uint256 value);

  constructor(string memory _name, string memory _symbol, uint8 _decimals) {
    name = _name;
    symbol = _symbol;
    decimals = _decimals;
  }

  /// @notice Mint tokens to an address.
  function mint(address to, uint256 amount) external {
    balanceOf[to] += amount;
    totalSupply += amount;
    emit Transfer(address(0), to, amount);
  }

  /// @notice A simple transfer function.
  /// SafeTransfer calls transfer under the hood.
  function transfer(address recipient, uint256 amount) external returns (bool) {
    require(balanceOf[msg.sender] >= amount, "insufficient balance");
    balanceOf[msg.sender] -= amount;
    balanceOf[recipient] += amount;
    emit Transfer(msg.sender, recipient, amount);
    return true;
  }

  // Minimal implementations to satisfy IERC20 interface.
  function approve(address, uint256) external pure returns (bool) {
    return true;
  }

  function transferFrom(address sender, address recipient, uint256 amount) external returns (bool) {
    require(balanceOf[sender] >= amount, "insufficient balance");
    balanceOf[sender] -= amount;
    balanceOf[recipient] += amount;
    emit Transfer(sender, recipient, amount);
    return true;
  }
}

/// @notice Foundry tests for the TokenAirdrop contract.
contract TokenAirdropTest is Test {
  TokenAirdrop public airdrop;
  TokenAirdrop public selfDestructedAirdrop;

  MockERC20 public token;
  MockERC20 public primaryFactor;
  MockERC20 public primaryConditional;
  MockERC20 public secondaryFactor;

  address public owner = address(0x123);
  address public user1 = address(0x456);
  address public user2 = address(0x789);

  uint256 public claimEnd;
  // Chosen multipliers such that (multiplier / DENOMINATOR) yields a whole number.
  uint256 primaryConditionalMultiplier = 5e9;
  uint256 DENOMINATOR = 1e9;

  uint256 primaryAllocation = (100e18 * primaryConditionalMultiplier) / DENOMINATOR;
  uint256 secondaryAllocation = 50e18;

  uint256 public defaultAllocation = primaryAllocation + secondaryAllocation;
  uint256 public fullAllocation = 1e30;

  function setUp() public {
    // Set claim period to 1 day in the future.
    claimEnd = block.timestamp + 1 days;
    // Deploy our mock tokens.
    token = new MockERC20("Token", "TKN", 18);
    primaryFactor = new MockERC20("PrimaryFactor", "PF", 18);
    primaryConditional = new MockERC20("PrimaryConditional", "PC", 18);
    secondaryFactor = new MockERC20("SecondaryFactor", "SF", 18);

    // Deploy the airdrop contract using both primary and secondary factors.
    airdrop = new TokenAirdrop(
      address(token),
      owner,
      claimEnd,
      address(primaryFactor),
      address(primaryConditional),
      address(secondaryFactor)
    );

    // Fund the airdrop contract so that it can pay out claims.
    token.mint(address(airdrop), 1e30);

    // mint 100
    primaryFactor.mint(user1, 100e18);
    primaryConditional.mint(user1, 5e9);
    secondaryFactor.mint(user1, 50e18);
  }

  function test_Succeeds_With_ConstructorEmittingAirdropConfigured() public {
    vm.expectEmit(true, true, true, true);
    emit TokenAirdrop.AirdropConfigured(
      address(token),
      address(primaryFactor),
      address(primaryConditional),
      address(secondaryFactor),
      claimEnd
    );

    // Deploy the airdrop contract using both primary and secondary factors.
    TokenAirdrop localAirdrop = new TokenAirdrop(
      address(token),
      owner,
      claimEnd,
      address(primaryFactor),
      address(primaryConditional),
      address(secondaryFactor)
    );

    assertEq(address(localAirdrop.TOKEN()), address(token));
    assertEq(address(localAirdrop.PRIMARY_FACTOR_ADDRESS()), address(primaryFactor));
    assertEq(address(localAirdrop.PRIMARY_CONDITIONAL_MULTIPLIER_ADDRESS()), address(primaryConditional));
    assertEq(address(localAirdrop.SECONDARY_FACTOR_ADDRESS()), address(secondaryFactor));
    assertEq(localAirdrop.CLAIM_END(), claimEnd);
    assertEq(localAirdrop.owner(), owner);
  }

  function test_Reverts_If_ConstructorHasZeroToken() public {
    vm.expectRevert(abi.encodeWithSelector(TokenAirdrop.ZeroAddressNotAllowed.selector));
    new TokenAirdrop(
      address(0),
      owner,
      claimEnd,
      address(primaryFactor),
      address(primaryConditional),
      address(secondaryFactor)
    );
  }

  function test_Reverts_If_ConstructorHasClaimEndTooShort() public {
    vm.expectRevert(abi.encodeWithSelector(TokenAirdrop.ClaimEndTooShort.selector));
    new TokenAirdrop(
      address(token),
      owner,
      block.timestamp,
      address(primaryFactor),
      address(primaryConditional),
      address(secondaryFactor)
    );
  }

  function test_Reverts_If_ConstructorHasZeroAddess() public {
    vm.expectRevert(abi.encodeWithSelector(Ownable.OwnableInvalidOwner.selector, address(0)));
    new TokenAirdrop(
      address(token),
      address(0),
      claimEnd,
      address(primaryFactor),
      address(primaryConditional),
      address(secondaryFactor)
    );
  }

  function test_Reverts_If_OnlyPrimaryAddressHasZeroAddess() public {
    vm.expectRevert(abi.encodeWithSelector(TokenAirdrop.ZeroAddressNotAllowed.selector));
    new TokenAirdrop(
      address(token),
      owner,
      claimEnd,
      address(0),
      address(primaryConditional),
      address(secondaryFactor)
    );
  }

  function test_Reverts_If_OnlyPrimaryConditionalAddressHasZeroAddess() public {
    vm.expectRevert(abi.encodeWithSelector(TokenAirdrop.ZeroAddressNotAllowed.selector));
    new TokenAirdrop(address(token), owner, claimEnd, address(primaryFactor), address(0), address(secondaryFactor));
  }

  function test_Reverts_If_ConstructorHasAllCalculationFactorsZero() public {
    vm.expectRevert(abi.encodeWithSelector(TokenAirdrop.AllCalculationFactorsZero.selector));
    new TokenAirdrop(address(token), owner, claimEnd, address(0), address(0), address(0));
  }

  function test_Succeeds_With_PrimaryOnlyUser1Claiming_AllSecondaryValuesZero() public {
    TokenAirdrop onlyPrimarAirdrop = new TokenAirdrop(
      address(token),
      owner,
      claimEnd,
      address(primaryFactor),
      address(primaryConditional),
      address(0)
    );

    vm.prank(owner);
    token.mint(address(onlyPrimarAirdrop), 1e30);

    vm.expectEmit(true, true, true, true);
    emit TokenAirdrop.Claimed(user1, primaryAllocation);

    vm.prank(user1);
    onlyPrimarAirdrop.claim();
    assertEq(token.balanceOf(user1), primaryAllocation);
  }

  function test_Reverts_If_User2ClaimsZeroTokens() public {
    address account = makeAddr("account");
    vm.expectRevert(abi.encodeWithSelector(TokenAirdrop.ClaimAmountIsZero.selector));
    vm.prank(account);
    airdrop.claim();
  }

  function test_Succeeds_With_PrimaryOnlyUser1Claiming_OnlySecondaryFactorZero() public {
    TokenAirdrop onlyPrimarAirdrop = new TokenAirdrop(
      address(token),
      owner,
      claimEnd,
      address(primaryFactor),
      address(primaryConditional),
      address(0)
    );

    vm.prank(owner);
    token.mint(address(onlyPrimarAirdrop), 1e30);

    vm.expectEmit(true, true, true, true);
    emit TokenAirdrop.Claimed(user1, primaryAllocation);

    vm.prank(user1);
    onlyPrimarAirdrop.claim();
    assertEq(token.balanceOf(user1), primaryAllocation);
  }

  function test_Succeeds_With_SecondaryOnlyUser1Claiming_AllPrimaryValuesZeros() public {
    TokenAirdrop onlySecondaryAirdrop = new TokenAirdrop(
      address(token),
      owner,
      claimEnd,
      address(0),
      address(0),
      address(secondaryFactor)
    );

    vm.prank(owner);
    token.mint(address(onlySecondaryAirdrop), 1e30);

    vm.expectEmit(true, true, true, true);
    emit TokenAirdrop.Claimed(user1, secondaryAllocation);

    vm.prank(user1);
    onlySecondaryAirdrop.claim();
    assertEq(token.balanceOf(user1), secondaryAllocation);
  }

  function test_Succeeds_With_SecondaryOnlyUser1Claiming_OnlyPrimaryFactorZero() public {
    TokenAirdrop onlySecondaryAirdrop = new TokenAirdrop(
      address(token),
      owner,
      claimEnd,
      address(0),
      address(0),
      address(secondaryFactor)
    );

    vm.prank(owner);
    token.mint(address(onlySecondaryAirdrop), 1e30);

    vm.expectEmit(true, true, true, true);
    emit TokenAirdrop.Claimed(user1, secondaryAllocation);

    vm.prank(user1);
    onlySecondaryAirdrop.claim();
    assertEq(token.balanceOf(user1), secondaryAllocation);
  }

  function test_Succeeds_With_SecondaryOnlyUser1Claiming_OnlyPrimaryConditionalMultiplierZero() public {
    TokenAirdrop onlySecondaryAirdrop = new TokenAirdrop(
      address(token),
      owner,
      claimEnd,
      address(0),
      address(0),
      address(secondaryFactor)
    );

    vm.prank(owner);
    token.mint(address(onlySecondaryAirdrop), 1e30);

    vm.expectEmit(true, true, true, true);
    emit TokenAirdrop.Claimed(user1, secondaryAllocation);

    vm.prank(user1);
    onlySecondaryAirdrop.claim();
    assertEq(token.balanceOf(user1), secondaryAllocation);
  }

  function test_Succeeds_With_User1AllocationCalculation() public view {
    assertEq(airdrop.calculateAllocation(user1), defaultAllocation);
  }

  function test_Succeeds_With_User2AllocationCalculation() public view {
    assertEq(airdrop.calculateAllocation(user2), 0);
  }

  function test_Succeeds_With_User1Claiming() public {
    assertEq(token.balanceOf(user1), 0);

    vm.prank(user1);
    airdrop.claim();
    assertEq(token.balanceOf(user1), defaultAllocation);
  }

  function test_Reverts_If_User1ClaimsTwice() public {
    vm.prank(user1);
    airdrop.claim();
    assertEq(token.balanceOf(user1), defaultAllocation);

    vm.prank(user1);
    vm.expectRevert(abi.encodeWithSelector(TokenAirdrop.AlreadyClaimed.selector));
    airdrop.claim();
  }

  function test_Reverts_If_User1ClaimsAfterClaimEnd() public {
    vm.warp(block.timestamp + 2 days);
    vm.expectRevert(abi.encodeWithSelector(TokenAirdrop.ClaimFinished.selector));
    vm.prank(user1);
    airdrop.claim();
  }

  function test_Reverts_If_OwnerWithdrawsBeforeClaimEnd() public {
    vm.expectRevert(abi.encodeWithSelector(TokenAirdrop.ClaimNotFinished.selector));
    vm.prank(owner);
    airdrop.withdraw();
  }

  function test_Reverts_If_NonOwnerWithdraws() public {
    vm.warp(block.timestamp + 2 days);
    vm.expectRevert(abi.encodeWithSelector(Ownable.OwnableUnauthorizedAccount.selector, user2));
    vm.prank(user2);
    airdrop.withdraw();
  }

  function test_Reverts_If_OwnerTriesToRenounceOwnership() public {
    vm.expectRevert(abi.encodeWithSelector(TokenAirdrop.RenouncingOwnershipDisabled.selector));
    vm.prank(owner);
    airdrop.renounceOwnership();
  }
}
