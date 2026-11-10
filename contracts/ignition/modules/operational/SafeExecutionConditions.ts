import { buildModule } from "@nomicfoundation/hardhat-ignition/modules";

/** Replaces `24_deploy_SafeExecutionConditions.ts` (tag `SafeExecutionConditions`). No parameters. */
export default buildModule("SafeExecutionConditions", (m) => {
  const safeExecutionConditions = m.contract("SafeExecutionConditions", [], { id: "SafeExecutionConditions" });
  return { safeExecutionConditions };
});
