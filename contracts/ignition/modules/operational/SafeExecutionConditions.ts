import { buildModule } from "@nomicfoundation/hardhat-ignition/modules";

/** No parameters. */
export default buildModule("SafeExecutionConditions", (m) => {
  const safeExecutionConditions = m.contract("SafeExecutionConditions", [], { id: "SafeExecutionConditions" });
  return { safeExecutionConditions };
});
