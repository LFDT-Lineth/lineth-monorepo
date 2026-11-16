import { buildModule } from "@nomicfoundation/hardhat-ignition/modules";

/** Parameters: `messageService`, `lineaToken`. */
export default buildModule("L1LineaTokenBurner", (m) => {
  const l1LineaTokenBurner = m.contract(
    "L1LineaTokenBurner",
    [m.getParameter<string>("messageService"), m.getParameter<string>("lineaToken")],
    { id: "L1LineaTokenBurner" },
  );
  return { l1LineaTokenBurner };
});
