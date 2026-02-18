// SPDX-License-Identifier: Apache-2.0 OR MIT
pragma solidity ^0.8.30;

import { console, Test } from "forge-std/Test.sol";
import { Initializable } from "@openzeppelin/contracts-upgradeable/proxy/utils/Initializable.sol";
import { IAccessControl } from "@openzeppelin/contracts/access/IAccessControl.sol";
import { ERC1967Proxy } from "@openzeppelin/contracts/proxy/ERC1967/ERC1967Proxy.sol";
import { LineaToken } from "src/L1/LineaToken.sol";
import { ILineaToken } from "src/L1/interfaces/ILineaToken.sol";
import { TestMessageService } from "../helpers/TestMessageService.sol";

contract LineaTokenTest is Test {
  LineaToken public tokenImpl;
  LineaToken public tokenProxy;
  address public l1MessageService;

  address public admin = makeAddr("admin");
  address public minter = makeAddr("minter");
  string public tokenName = "Linea";
  string public tokenSymbol = "LTK";
  address l2TokenAddress = makeAddr("l2Token");

  function setUp() public virtual {
    l1MessageService = address(new TestMessageService());
    tokenImpl = new LineaToken();

    bytes memory data = abi.encodeWithSelector(
      LineaToken.initialize.selector,
      admin,
      minter,
      l1MessageService,
      l2TokenAddress,
      tokenName,
      tokenSymbol
    );

    vm.expectEmit(true, true, true, false);
    emit ILineaToken.L2TokenAddressSet(l2TokenAddress);

    tokenProxy = LineaToken(address(new ERC1967Proxy(address(tokenImpl), data)));
  }

  function test_shouldEmitL2TokenAddressSetOnInitialize() public {
    address localL1MessageService = address(new TestMessageService());
    LineaToken localTokenImpl = new LineaToken();

    bytes memory data = abi.encodeWithSelector(
      LineaToken.initialize.selector,
      admin,
      minter,
      localL1MessageService,
      l2TokenAddress,
      tokenName,
      tokenSymbol
    );

    vm.expectEmit(true, true, true, false);
    emit ILineaToken.L2TokenAddressSet(l2TokenAddress);

    LineaToken(address(new ERC1967Proxy(address(localTokenImpl), data)));
  }

  function test_shouldEmitL1MessageServiceSetOnInitialize() public {
    vm.expectEmit(true, true, true, true);
    emit ILineaToken.L1MessageServiceSet(l1MessageService);

    bytes memory data = abi.encodeWithSelector(
      LineaToken.initialize.selector,
      admin,
      minter,
      l1MessageService,
      l2TokenAddress,
      tokenName,
      tokenSymbol
    );

    LineaToken(address(new ERC1967Proxy(address(tokenImpl), data)));
  }

  function test_shouldEmitTokenMetadataSetOnInitialize() public {
    vm.expectEmit(true, true, true, true);
    emit ILineaToken.TokenMetadataSet(tokenName, tokenSymbol);

    bytes memory data = abi.encodeWithSelector(
      LineaToken.initialize.selector,
      admin,
      minter,
      l1MessageService,
      l2TokenAddress,
      tokenName,
      tokenSymbol
    );

    LineaToken(address(new ERC1967Proxy(address(tokenImpl), data)));
  }

  function test_shouldInitializeLineaTokenCorrectly() public view {
    assertEq(tokenProxy.name(), tokenName);
    assertEq(tokenProxy.symbol(), tokenSymbol);
    assertTrue(tokenProxy.hasRole(tokenProxy.DEFAULT_ADMIN_ROLE(), admin));
    assertTrue(tokenProxy.hasRole(tokenProxy.MINTER_ROLE(), minter));
    assertEq(tokenProxy.l1MessageService(), l1MessageService);
    assertEq(tokenProxy.l2TokenAddress(), l2TokenAddress);
  }

  function test_shouldNotInitializeTwice() public {
    vm.expectRevert(abi.encodeWithSelector(Initializable.InvalidInitialization.selector));
    tokenProxy.initialize(admin, minter, l1MessageService, l2TokenAddress, tokenName, tokenSymbol);
  }

  function test_shouldRevertWhenTokenNameIsEmpty() public {
    LineaToken implementation = new LineaToken();

    bytes memory data = abi.encodeWithSelector(
      LineaToken.initialize.selector,
      admin,
      minter,
      l1MessageService,
      l2TokenAddress,
      "",
      tokenSymbol
    );

    vm.expectRevert(abi.encodeWithSelector(ILineaToken.EmptyStringNotAllowed.selector));
    tokenProxy = LineaToken(address(new ERC1967Proxy(address(implementation), data)));
  }

  function test_shouldRevertWhenTokenSymbolIsEmpty() public {
    LineaToken implementation = new LineaToken();

    bytes memory data = abi.encodeWithSelector(
      LineaToken.initialize.selector,
      admin,
      minter,
      l1MessageService,
      l2TokenAddress,
      tokenName,
      ""
    );

    vm.expectRevert(abi.encodeWithSelector(ILineaToken.EmptyStringNotAllowed.selector));
    tokenProxy = LineaToken(address(new ERC1967Proxy(address(implementation), data)));
  }

  function test_shouldRevertWhenDefaultAdminIsZeroAddress() public {
    LineaToken implementation = new LineaToken();

    address defaultAdmin = address(0);
    bytes memory data = abi.encodeWithSelector(
      LineaToken.initialize.selector,
      defaultAdmin,
      minter,
      l1MessageService,
      l2TokenAddress,
      tokenName,
      tokenSymbol
    );

    vm.expectRevert(abi.encodeWithSelector(ILineaToken.ZeroAddressNotAllowed.selector));
    tokenProxy = LineaToken(address(new ERC1967Proxy(address(implementation), data)));
  }

  function test_shouldRevertWhenMinterIsZeroAddress() public {
    LineaToken implementation = new LineaToken();

    address minterAddress = address(0);
    bytes memory data = abi.encodeWithSelector(
      LineaToken.initialize.selector,
      admin,
      minterAddress,
      l1MessageService,
      l2TokenAddress,
      tokenName,
      tokenSymbol
    );

    vm.expectRevert(abi.encodeWithSelector(ILineaToken.ZeroAddressNotAllowed.selector));
    tokenProxy = LineaToken(address(new ERC1967Proxy(address(implementation), data)));
  }

  function test_shouldRevertWhenL1MessageServiceIsZeroAddress() public {
    LineaToken implementation = new LineaToken();

    address l1MessageServiceAddress = address(0);
    bytes memory data = abi.encodeWithSelector(
      LineaToken.initialize.selector,
      admin,
      minter,
      l1MessageServiceAddress,
      l2TokenAddress,
      tokenName,
      tokenSymbol
    );

    vm.expectRevert(abi.encodeWithSelector(ILineaToken.ZeroAddressNotAllowed.selector));
    tokenProxy = LineaToken(address(new ERC1967Proxy(address(implementation), data)));
  }

  function test_shouldRevertWhenL2TokenAddressIsZeroAddress() public {
    LineaToken implementation = new LineaToken();

    address l2Token = address(0);
    bytes memory data = abi.encodeWithSelector(
      LineaToken.initialize.selector,
      admin,
      minter,
      l1MessageService,
      l2Token,
      tokenName,
      tokenSymbol
    );

    vm.expectRevert(abi.encodeWithSelector(ILineaToken.ZeroAddressNotAllowed.selector));
    tokenProxy = LineaToken(address(new ERC1967Proxy(address(implementation), data)));
  }

  function test_shouldMintTokens() public {
    address account = makeAddr("account");
    uint256 amount = 1e18;

    vm.prank(minter);
    tokenProxy.mint(account, amount);

    assertEq(tokenProxy.balanceOf(account), amount);
    assertEq(tokenProxy.totalSupply(), amount);
  }

  function test_shouldStartSyncingTotalSupplyToL2() public {
    address account = makeAddr("minter");

    vm.prank(minter);
    tokenProxy.mint(account, 1e18);

    uint256 totalSupply = tokenProxy.totalSupply();
    assertEq(totalSupply, 1e18);

    vm.expectEmit(true, true, true, true);
    emit ILineaToken.L1TotalSupplySyncStarted(totalSupply);

    tokenProxy.syncTotalSupplyToL2();
  }
}
