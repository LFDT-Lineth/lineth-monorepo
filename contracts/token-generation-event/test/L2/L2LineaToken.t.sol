// SPDX-License-Identifier: Apache-2.0 OR MIT
pragma solidity ^0.8.30;

import { console, Test } from "forge-std/Test.sol";
import { Initializable } from "@openzeppelin/contracts-upgradeable/proxy/utils/Initializable.sol";
import { IAccessControl } from "@openzeppelin/contracts/access/IAccessControl.sol";
import { ERC1967Proxy } from "@openzeppelin/contracts/proxy/ERC1967/ERC1967Proxy.sol";
import { IVotes } from "@openzeppelin/contracts/governance/utils/IVotes.sol";
import { L2LineaToken } from "src/L2/L2LineaToken.sol";
import { IL2LineaToken } from "src/L2/interfaces/IL2LineaToken.sol";
import { TestL2MessageService } from "../helpers/TestL2MessageService.sol";
import { IGenericErrors } from "src/interfaces/IGenericErrors.sol";
import { MessageServiceBase } from "src/MessageServiceBase.sol";

contract L2LineaTokenTest is Test {
  L2LineaToken public tokenImpl;
  L2LineaToken public tokenProxy;

  TestL2MessageService public lineaMessageService;
  address public lineaMessageServiceAddress;

  address public admin = makeAddr("admin");
  address public lineaCanonicalTokenBridge = makeAddr("lineaCanonicalTokenBridge");
  address public l1Token = makeAddr("l1Token");
  string public tokenName = "L2Linea";
  string public tokenSymbol = "L2LTK";

  function setUp() public virtual {
    lineaMessageService = new TestL2MessageService();
    lineaMessageServiceAddress = address(lineaMessageService);
    tokenImpl = new L2LineaToken();

    bytes memory data = abi.encodeWithSelector(
      L2LineaToken.initialize.selector,
      admin,
      lineaCanonicalTokenBridge,
      lineaMessageServiceAddress,
      l1Token,
      tokenName,
      tokenSymbol
    );

    tokenProxy = L2LineaToken(address(new ERC1967Proxy(address(tokenImpl), data)));
  }

  function test_shouldEmitLineaCanonicalTokenBridgeSet() public {
    vm.expectEmit(true, true, true, true);
    emit IL2LineaToken.LineaCanonicalTokenBridgeSet(lineaCanonicalTokenBridge);

    bytes memory data = abi.encodeWithSelector(
      L2LineaToken.initialize.selector,
      admin,
      lineaCanonicalTokenBridge,
      lineaMessageServiceAddress,
      l1Token,
      tokenName,
      tokenSymbol
    );

    L2LineaToken(address(new ERC1967Proxy(address(tokenImpl), data)));
  }

  function test_shouldEmitL2MessageServiceSet() public {
    vm.expectEmit(true, true, true, true);
    emit IL2LineaToken.L2MessageServiceSet(lineaMessageServiceAddress);

    bytes memory data = abi.encodeWithSelector(
      L2LineaToken.initialize.selector,
      admin,
      lineaCanonicalTokenBridge,
      lineaMessageServiceAddress,
      l1Token,
      tokenName,
      tokenSymbol
    );

    L2LineaToken(address(new ERC1967Proxy(address(tokenImpl), data)));
  }

  function test_shouldEmitTokenMetadataSetOnInitialize() public {
    vm.expectEmit(true, true, true, true);
    emit IL2LineaToken.TokenMetadataSet(tokenName, tokenSymbol);

    bytes memory data = abi.encodeWithSelector(
      L2LineaToken.initialize.selector,
      admin,
      lineaCanonicalTokenBridge,
      lineaMessageServiceAddress,
      l1Token,
      tokenName,
      tokenSymbol
    );

    L2LineaToken(address(new ERC1967Proxy(address(tokenImpl), data)));
  }

  function test_shouldInitializeL2LineaTokenCorrectly() public view {
    assertEq(tokenProxy.name(), tokenName);
    assertEq(tokenProxy.symbol(), tokenSymbol);
    assertEq(tokenProxy.decimals(), 18);
    assertTrue(tokenProxy.hasRole(tokenProxy.DEFAULT_ADMIN_ROLE(), admin));
    assertEq(address(tokenProxy.messageService()), lineaMessageServiceAddress);
    assertEq(tokenProxy.remoteSender(), l1Token);
    assertEq(tokenProxy.lineaCanonicalTokenBridge(), lineaCanonicalTokenBridge);
  }

  function test_shouldEmitL1TokenAddressSetOnInitialize() public {
    address localLineaMessageService = address(new TestL2MessageService());
    L2LineaToken localTokenImpl = new L2LineaToken();

    bytes memory data = abi.encodeWithSelector(
      L2LineaToken.initialize.selector,
      admin,
      lineaCanonicalTokenBridge,
      localLineaMessageService,
      l1Token,
      tokenName,
      tokenSymbol
    );

    vm.expectEmit(true, true, true, false);
    emit IL2LineaToken.L1TokenAddressSet(l1Token);

    L2LineaToken(address(new ERC1967Proxy(address(localTokenImpl), data)));
  }

  function test_shouldNotInitializeTwice() public {
    vm.expectRevert(abi.encodeWithSelector(Initializable.InvalidInitialization.selector));
    tokenProxy.initialize(
      admin,
      lineaCanonicalTokenBridge,
      lineaMessageServiceAddress,
      l1Token,
      tokenName,
      tokenSymbol
    );
  }

  function test_shouldRevertWhenTokenNameIsEmpty() public {
    L2LineaToken implementation = new L2LineaToken();

    bytes memory data = abi.encodeWithSelector(
      L2LineaToken.initialize.selector,
      admin,
      lineaCanonicalTokenBridge,
      lineaMessageServiceAddress,
      l1Token,
      "",
      tokenSymbol
    );

    vm.expectRevert(abi.encodeWithSelector(IL2LineaToken.EmptyStringNotAllowed.selector));
    tokenProxy = L2LineaToken(address(new ERC1967Proxy(address(implementation), data)));
  }

  function test_shouldRevertWhenTokenSymbolIsEmpty() public {
    L2LineaToken implementation = new L2LineaToken();

    bytes memory data = abi.encodeWithSelector(
      L2LineaToken.initialize.selector,
      admin,
      lineaCanonicalTokenBridge,
      lineaMessageServiceAddress,
      l1Token,
      tokenName,
      ""
    );

    vm.expectRevert(abi.encodeWithSelector(IL2LineaToken.EmptyStringNotAllowed.selector));
    tokenProxy = L2LineaToken(address(new ERC1967Proxy(address(implementation), data)));
  }

  function test_shouldRevertWhenDefaultAdminIsZeroAddress() public {
    L2LineaToken implementation = new L2LineaToken();

    address defaultAdmin = address(0);
    bytes memory data = abi.encodeWithSelector(
      L2LineaToken.initialize.selector,
      defaultAdmin,
      lineaCanonicalTokenBridge,
      lineaMessageServiceAddress,
      l1Token,
      tokenName,
      tokenSymbol
    );

    vm.expectRevert(abi.encodeWithSelector(IGenericErrors.ZeroAddressNotAllowed.selector));
    tokenProxy = L2LineaToken(address(new ERC1967Proxy(address(implementation), data)));
  }

  function test_shouldRevertWhenLineaCanonicalTokenBridgeIsZeroAddress() public {
    L2LineaToken implementation = new L2LineaToken();

    address lineaCanonicalTokenBridgeAddress = address(0);
    bytes memory data = abi.encodeWithSelector(
      L2LineaToken.initialize.selector,
      admin,
      lineaCanonicalTokenBridgeAddress,
      lineaMessageServiceAddress,
      l1Token,
      tokenName,
      tokenSymbol
    );

    vm.expectRevert(abi.encodeWithSelector(IGenericErrors.ZeroAddressNotAllowed.selector));
    tokenProxy = L2LineaToken(address(new ERC1967Proxy(address(implementation), data)));
  }

  function test_shouldRevertWhenL2MessageServiceIsZeroAddress() public {
    L2LineaToken implementation = new L2LineaToken();

    address lineaMessageServiceEmpty = address(0);
    bytes memory data = abi.encodeWithSelector(
      L2LineaToken.initialize.selector,
      admin,
      lineaCanonicalTokenBridge,
      lineaMessageServiceEmpty,
      l1Token,
      tokenName,
      tokenSymbol
    );

    vm.expectRevert(abi.encodeWithSelector(IGenericErrors.ZeroAddressNotAllowed.selector));
    tokenProxy = L2LineaToken(address(new ERC1967Proxy(address(implementation), data)));
  }

  function test_shouldRevertWhenL1TokenIsZeroAddress() public {
    L2LineaToken implementation = new L2LineaToken();

    address l1TokenAddress = address(0);
    bytes memory data = abi.encodeWithSelector(
      L2LineaToken.initialize.selector,
      admin,
      lineaCanonicalTokenBridge,
      lineaMessageServiceAddress,
      l1TokenAddress,
      tokenName,
      tokenSymbol
    );

    vm.expectRevert(abi.encodeWithSelector(IGenericErrors.ZeroAddressNotAllowed.selector));
    tokenProxy = L2LineaToken(address(new ERC1967Proxy(address(implementation), data)));
  }

  function test_shouldRevertMintingWhenCallerIsNotLineaCanonicalTokenBridge() public {
    address nonBridgeCaller = makeAddr("nonBridgeCaller");

    vm.prank(nonBridgeCaller);
    vm.expectRevert(abi.encodeWithSelector(IL2LineaToken.CallerIsNotTokenBridge.selector));
    tokenProxy.mint(makeAddr("account"), 1e18);
  }

  function test_shouldMintTokens() public {
    address account = makeAddr("account");
    uint256 amount = 1e18;

    vm.prank(lineaCanonicalTokenBridge);
    tokenProxy.mint(account, amount);

    assertEq(tokenProxy.balanceOf(account), amount);
    assertEq(tokenProxy.totalSupply(), amount);
  }

  function test_shouldUpdateDelegateVotesWhenMinting() public {
    address account = makeAddr("account");
    uint256 amount = 1e18;

    address delegatee = makeAddr("delegatee");
    vm.prank(account);
    tokenProxy.delegate(delegatee);

    vm.expectEmit(true, true, true, true);
    emit IVotes.DelegateVotesChanged(delegatee, 0, amount);
    vm.prank(lineaCanonicalTokenBridge);
    tokenProxy.mint(account, amount);

    assertEq(tokenProxy.balanceOf(account), amount);
    assertEq(tokenProxy.totalSupply(), amount);
  }

  function test_shouldRevertBurningWhenCallerIsNotLineaCanonicalTokenBridge() public {
    address nonBridgeCaller = makeAddr("nonBridgeCaller");
    address account = makeAddr("account");
    uint256 amount = 1e18;

    vm.prank(nonBridgeCaller);
    vm.expectRevert(abi.encodeWithSelector(IL2LineaToken.CallerIsNotTokenBridge.selector));
    tokenProxy.burn(account, amount);
  }

  function test_shouldBurnTokensUsingLineaCanonicalTokenBridge() public {
    address account = makeAddr("account");
    uint256 amount = 1e18;

    vm.prank(lineaCanonicalTokenBridge);
    tokenProxy.mint(account, amount);

    vm.prank(account);
    tokenProxy.approve(lineaCanonicalTokenBridge, amount);

    assertEq(tokenProxy.balanceOf(account), amount);
    assertEq(tokenProxy.totalSupply(), amount);

    vm.prank(lineaCanonicalTokenBridge);
    tokenProxy.burn(account, amount);

    assertEq(tokenProxy.balanceOf(account), 0);
    assertEq(tokenProxy.totalSupply(), 0);
  }

  function test_shouldRevertWhenCallerIsNotLineaMessageService() public {
    address nonMessageServiceCaller = makeAddr("nonMessageServiceCaller");

    vm.prank(nonMessageServiceCaller);
    vm.expectRevert(abi.encodeWithSelector(MessageServiceBase.CallerIsNotMessageService.selector));
    tokenProxy.syncTotalSupplyFromL1(block.timestamp, 1000);
  }

  function test_shouldRevertWhenSenderIsNotAuthorized() public {
    uint256 l1LineaTokenTotalSupplySyncTime = block.timestamp;
    uint256 l1LineaTokenSupply = 1000;

    bytes memory data = abi.encodeCall(
      L2LineaToken.syncTotalSupplyFromL1,
      (l1LineaTokenTotalSupplySyncTime, l1LineaTokenSupply)
    );
    address _to = address(tokenProxy);
    address wrongOriginalSender = makeAddr("wrongOriginalSender");

    vm.expectRevert(abi.encodeWithSelector(MessageServiceBase.SenderNotAuthorized.selector));
    lineaMessageService.syncL1TotalSupply(wrongOriginalSender, _to, 0, data);
  }

  function test_shouldSyncTotalSupplyFromL1() public {
    vm.warp(block.timestamp + 5);
    uint256 l1LineaTokenTotalSupplySyncTime = block.timestamp - 5;
    uint256 l1LineaTokenSupply = 1000;

    bytes memory data = abi.encodeCall(
      L2LineaToken.syncTotalSupplyFromL1,
      (l1LineaTokenTotalSupplySyncTime, l1LineaTokenSupply)
    );
    address _to = address(tokenProxy);

    vm.expectEmit(true, true, true, true);
    emit IL2LineaToken.L1LineaTokenTotalSupplySynced(l1LineaTokenTotalSupplySyncTime, l1LineaTokenSupply);

    lineaMessageService.syncL1TotalSupply(l1Token, _to, 0, data);

    assertEq(tokenProxy.l1LineaTokenSupply(), l1LineaTokenSupply);
    assertEq(tokenProxy.l1LineaTokenTotalSupplySyncTime(), l1LineaTokenTotalSupplySyncTime);
  }

  function test_shouldFailOnOldSync() public {
    vm.warp(block.timestamp + 5);
    uint256 l1LineaTokenTotalSupplySyncTime = block.timestamp - 3;
    uint256 l1LineaTokenSupply = 1000;

    bytes memory data = abi.encodeCall(
      L2LineaToken.syncTotalSupplyFromL1,
      (l1LineaTokenTotalSupplySyncTime, l1LineaTokenSupply)
    );
    address _to = address(tokenProxy);

    vm.expectEmit(true, true, true, true);
    emit IL2LineaToken.L1LineaTokenTotalSupplySynced(l1LineaTokenTotalSupplySyncTime, l1LineaTokenSupply);

    lineaMessageService.syncL1TotalSupply(l1Token, _to, 0, data);

    data = abi.encodeCall(
      L2LineaToken.syncTotalSupplyFromL1,
      (l1LineaTokenTotalSupplySyncTime - 1, l1LineaTokenSupply)
    );

    vm.expectRevert(abi.encodeWithSelector(IL2LineaToken.LastSyncMoreRecent.selector));
    lineaMessageService.syncL1TotalSupply(l1Token, _to, 0, data);

    assertEq(tokenProxy.l1LineaTokenSupply(), l1LineaTokenSupply);
    assertEq(tokenProxy.l1LineaTokenTotalSupplySyncTime(), l1LineaTokenTotalSupplySyncTime);
  }
}
