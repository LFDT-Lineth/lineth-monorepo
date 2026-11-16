import { buildModule } from "@nomicfoundation/hardhat-ignition/modules";

import { UNSAFE_ALLOW, deployPinnedTransparentProxy } from "../lib/proxy.js";

/**
 * Implementation + pinned OZ 4.x proxy,
 * `initializeRolesAndStorageVariables(...)`.
 *
 * Parameters: `lastInvoiceDate`, `securityCouncil`, `invoiceSubmitter`, `burner`, `invoicePaymentReceiver`, `tokenBridge`,
 * `messageService`, `l1LineaTokenBurner`, `lineaToken`, `dexSwapAdapter`.
 */
export default buildModule("RollupRevenueVault", (m) => {
  const implementation = m.contract("RollupRevenueVault", [], { id: "RollupRevenueVaultImplementation" });
  const { proxy, proxyAdmin } = deployPinnedTransparentProxy(m, {
    name: "RollupRevenueVault",
    unsafeAllow: UNSAFE_ALLOW.RollupRevenueVault,
    implementation,
    initializer: "initializeRolesAndStorageVariables",
    initializerArgs: [
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
  });
  const rollupRevenueVault = m.contractAt("RollupRevenueVault", proxy, { id: "RollupRevenueVault" });
  return {
    rollupRevenueVault,
    rollupRevenueVaultImplementation: implementation,
    rollupRevenueVaultProxy: proxy,
    proxyAdmin,
  };
});
