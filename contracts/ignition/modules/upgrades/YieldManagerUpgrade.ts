import { buildModule } from "@nomicfoundation/hardhat-ignition/modules";
import { zeroAddress } from "viem";

import { deployUpgradeImplementation } from "../lib/proxy.js";

/** New YieldManager implementation (constructor `linethRollup`) for the existing proxy (`ProxyAdmin.upgrade` calldata). Parameters: `proxy`, `linethRollup`. */
export default buildModule("YieldManagerUpgrade", (m) => {
  const implementation = deployUpgradeImplementation(m, {
    contractName: "YieldManager",
    unsafeAllow: ["constructor", "incorrect-initializer-order", "state-variable-immutable", "delegatecall"],
    constructorArgs: [m.getParameter<string>("linethRollup")],
    validationConstructorArgs: [zeroAddress],
  });
  return { implementation };
});
