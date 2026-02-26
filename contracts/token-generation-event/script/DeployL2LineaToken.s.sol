// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import { Script, console } from "forge-std/Script.sol";
import { Vm } from "forge-std/Vm.sol";
import { TransparentUpgradeableProxy } from "@openzeppelin/contracts/proxy/transparent/TransparentUpgradeableProxy.sol";
import { L2LineaToken } from "src/L2/L2LineaToken.sol";

contract DeployL2LineaToken is Script {
  address proxyAdminOwner;
  address tokenAdmin;
  address lineaCanonicalTokenBridge;
  address lineaMessageService;
  address l1LineaToken;
  string tokenName;
  string tokenSymbol;

  L2LineaToken proxy;

  function setUp() public virtual {
    console.log("Deployer address:\t", msg.sender);
    proxyAdminOwner = vm.envAddress("PROXY_ADMIN_OWNER_ADDRESS");
    console.log("Proxy admin address:\t", proxyAdminOwner);

    tokenAdmin = vm.envAddress("TOKEN_ADMIN_ADDRESS");
    console.log("Token admin address:\t", tokenAdmin);

    lineaCanonicalTokenBridge = vm.envAddress("LINEA_CANONICAL_TOKEN_BRIDGE");
    console.log("Linea Canonical Token Bridge address:\t", lineaCanonicalTokenBridge);

    lineaMessageService = vm.envAddress("LINEA_MESSAGE_SERVICE_ADDRESS");
    console.log("Linea Message Service address:\t", lineaMessageService);

    l1LineaToken = vm.envAddress("L1_LINEA_TOKEN_ADDRESS");
    console.log("L1 Linea Token address:\t", l1LineaToken);

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
    L2LineaToken token = new L2LineaToken();

    console.log("L2LineaToken impl:\t", address(token));
    proxy = L2LineaToken(
      address(
        new TransparentUpgradeableProxy(
          address(token),
          proxyAdminOwner,
          abi.encodeWithSelector(
            token.initialize.selector,
            tokenAdmin,
            lineaCanonicalTokenBridge,
            lineaMessageService,
            l1LineaToken,
            tokenName,
            tokenSymbol
          )
        )
      )
    );

    console.log("L2LineaToken proxy:\t", address(proxy));
    vm.stopBroadcast();
  }
}
