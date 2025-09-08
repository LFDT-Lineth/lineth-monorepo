// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import {Script, console} from "forge-std/Script.sol";
import {Vm} from "forge-std/Vm.sol";
import {TransparentUpgradeableProxy} from "@openzeppelin/contracts/proxy/transparent/TransparentUpgradeableProxy.sol";
import {LineaToken} from "src/L1/LineaToken.sol";

contract DeployLineaToken is Script {
  address proxyAdminOwner;
  address tokenAdmin;
  address minter;
  address l1MessageService;
  address l2LineaTokenAddress;
  string tokenName;
  string tokenSymbol;

  LineaToken proxy;

  function setUp() public virtual {
    console.log("Deployer address:\t", msg.sender);
    proxyAdminOwner = vm.envAddress("PROXY_ADMIN_OWNER_ADDRESS");
    console.log("Proxy admin address:\t", proxyAdminOwner);

    tokenAdmin = vm.envAddress("TOKEN_ADMIN_ADDRESS");
    console.log("Token admin address:\t", tokenAdmin);

    minter = vm.envAddress("MINTER_ADDRESS");
    console.log("Minter admin address:\t", minter);

    l1MessageService = vm.envAddress("L1_MESSAGE_SERVICE_ADDRESS");
    console.log("L1 Message Service address:\t", l1MessageService);

    l2LineaTokenAddress = vm.envAddress("L2_LINEA_TOKEN_ADDRESS");
    console.log("L2 Linea Token address:\t", l2LineaTokenAddress);

    tokenName = vm.envString("TOKEN_NAME");
    console.log("Token name:\t", tokenName);

    tokenSymbol = vm.envString("TOKEN_SYMBOL");
    console.log("Token symbol:\t", tokenSymbol);

    if (proxyAdminOwner == tokenAdmin) {
      revert("Proxy admin and token admin must be different");
    }
  }

  function run() public virtual {
    vm.startBroadcast();
    LineaToken token = new LineaToken();

    console.log("Lineatoken impl:\t", address(token));
    proxy = LineaToken(
      address(
        new TransparentUpgradeableProxy(
          address(token),
          proxyAdminOwner,
          abi.encodeWithSelector(token.initialize.selector, tokenAdmin, minter, l1MessageService, l2LineaTokenAddress, tokenName, tokenSymbol)
        )
      )
    );

    console.log("LineaToken proxy:\t", address(proxy));
    vm.stopBroadcast();
  }
}
