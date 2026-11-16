import { buildModule } from "@nomicfoundation/hardhat-ignition/modules";

/** Parameter: `target` (the LinethRollup). */
export default buildModule("CallForwardingProxy", (m) => {
  const callForwardingProxy = m.contract("CallForwardingProxy", [m.getParameter<string>("target")], {
    id: "CallForwardingProxy",
  });
  return { callForwardingProxy };
});
