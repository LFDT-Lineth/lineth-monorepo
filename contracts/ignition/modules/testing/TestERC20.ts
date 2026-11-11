import { buildModule } from "@nomicfoundation/hardhat-ignition/modules";

/** Replaces `11_deploy_TestERC20.ts` (tag `TestERC20`). Parameters: `name`, `symbol`, `initialSupply` (wei). */
export default buildModule("TestERC20", (m) => {
  const testERC20 = m.contract(
    "TestERC20",
    [m.getParameter<string>("name"), m.getParameter<string>("symbol"), m.getParameter("initialSupply")],
    { id: "TestERC20" },
  );
  return { testERC20 };
});
