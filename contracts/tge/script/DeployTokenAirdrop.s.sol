// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import { Script, console } from "forge-std/Script.sol";
import { Vm } from "forge-std/Vm.sol";
import { TokenAirdrop } from "src/airdrops/TokenAirdrop.sol";

contract DeployTokenAirdrop is Script {
  address token;
  address ownerAddress;
  uint256 claimEnd;
  address primaryFactorAddress;
  address primaryConditionalMultiplierAddress;
  address secondaryFactorAddress;

  function setUp() public virtual {
    console.log("Deployer address:\t", msg.sender);

    token = vm.envAddress("TOKEN_ADDRESS");
    console.log("Token address:\t", token);

    ownerAddress = vm.envAddress("OWNER_ADDRESS");
    console.log("Owner address:\t", ownerAddress);

    claimEnd = vm.envUint("CLAIM_END");
    console.log("Claim end:\t", claimEnd);

    primaryFactorAddress = vm.envAddress("PRIMARY_FACTOR_ADDRESS");
    console.log("Primary factor address:\t", primaryFactorAddress);

    primaryConditionalMultiplierAddress = vm.envAddress("PRIMARY_CONDITIONAL_MULTIPLIER_ADDRESS");
    console.log("Primary conditional multiplier address:\t", primaryConditionalMultiplierAddress);

    secondaryFactorAddress = vm.envAddress("SECONDARY_FACTOR_ADDRESS");
    console.log("Secondary factor address:\t", secondaryFactorAddress);
  }

  function run() public virtual {
    vm.startBroadcast();
    TokenAirdrop tokenAirdrop = new TokenAirdrop(
      token,
      ownerAddress,
      claimEnd,
      primaryFactorAddress,
      primaryConditionalMultiplierAddress,
      secondaryFactorAddress
    );

    console.log("TokenAirdrop contract address:\t", address(tokenAirdrop));

    vm.stopBroadcast();
  }
}
