import { buildModule } from "@nomicfoundation/hardhat-ignition/modules";

/**
 * Replaces `20_deploy_V3DexSwapAdapter.ts` (tag `V3DexSwapAdapter`).
 * Parameters: `router`, `wethToken`, `lineaToken`, `poolTickSpacing`.
 */
export default buildModule("V3DexSwapAdapter", (m) => {
  const v3DexSwapAdapter = m.contract(
    "V3DexSwapAdapter",
    [
      m.getParameter<string>("router"),
      m.getParameter<string>("wethToken"),
      m.getParameter<string>("lineaToken"),
      m.getParameter("poolTickSpacing"),
    ],
    { id: "V3DexSwapAdapter" },
  );
  return { v3DexSwapAdapter };
});
