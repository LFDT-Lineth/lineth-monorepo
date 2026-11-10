import { buildModule } from "@nomicfoundation/hardhat-ignition/modules";

import { deployUpgradeImplementation } from "../lib/proxy.js";

/** New RollupRevenueVault implementation + `initializeRolesAndStorageVariables(...)` (`ProxyAdmin.upgradeAndCall` calldata). Parameters: `proxy`, `lastInvoiceDate`, `securityCouncil`, `invoiceSubmitter`, `burner`, `invoicePaymentReceiver`, `tokenBridge`, `messageService`, `l1LineaTokenBurner`, `lineaToken`, `dexSwapAdapter`. */
export default buildModule("RollupRevenueVaultReinitialize", (m) => {
  const implementation = deployUpgradeImplementation(m, {
    contractName: "RollupRevenueVault",
    unsafeAllow: ["constructor", "incorrect-initializer-order"],
    reinitializer: {
      functionName: "initializeRolesAndStorageVariables",
      args: [
        m.getParameter("lastInvoiceDate"),
        m.getParameter<string>("securityCouncil"),
        m.getParameter<string>("invoiceSubmitter"),
        m.getParameter<string>("burner"),
        m.getParameter<string>("invoicePaymentReceiver"),
        m.getParameter<string>("tokenBridge"),
        m.getParameter<string>("messageService"),
        m.getParameter<string>("l1LineaTokenBurner"),
        m.getParameter<string>("lineaToken"),
        m.getParameter<string>("dexSwapAdapter"),
      ],
    },
  });
  return { implementation };
});
