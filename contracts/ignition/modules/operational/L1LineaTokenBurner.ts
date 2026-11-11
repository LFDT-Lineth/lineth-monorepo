import { buildModule } from "@nomicfoundation/hardhat-ignition/modules";

/** Replaces `19_deploy_L1LineaTokenBurner.ts` (tag `L1LineaTokenBurner`). Parameters: `messageService`, `lineaToken`. */
export default buildModule("L1LineaTokenBurner", (m) => {
  const l1LineaTokenBurner = m.contract(
    "L1LineaTokenBurner",
    [m.getParameter<string>("messageService"), m.getParameter<string>("lineaToken")],
    { id: "L1LineaTokenBurner" },
  );
  return { l1LineaTokenBurner };
});
