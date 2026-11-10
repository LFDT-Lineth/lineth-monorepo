import { buildModule } from "@nomicfoundation/hardhat-ignition/modules";

/** Replaces `12_deploy_CallForwardingProxy.ts` (tag `CallForwardingProxy`). Parameter: `target` (the LinethRollup). */
export default buildModule("CallForwardingProxy", (m) => {
  const callForwardingProxy = m.contract("CallForwardingProxy", [m.getParameter<string>("target")], {
    id: "CallForwardingProxy",
  });
  return { callForwardingProxy };
});
