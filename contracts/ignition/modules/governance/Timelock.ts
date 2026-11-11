import { buildModule } from "@nomicfoundation/hardhat-ignition/modules";

import type { IgnitionModuleBuilder } from "@nomicfoundation/ignition-core";

/** Parameters: `timelockMinDelay`, `timelockProposers`, `timelockExecutors`, `timelockAdmin`. */
export function addTimelock(m: IgnitionModuleBuilder) {
  const timelock = m.contract(
    "TimeLock",
    [
      m.getParameter("timelockMinDelay"),
      m.getParameter("timelockProposers"),
      m.getParameter("timelockExecutors"),
      m.getParameter<string>("timelockAdmin"),
    ],
    { id: "TimeLock" },
  );
  return { timelock };
}

/** Replaces `02_deploy_Timelock.ts` (tag `Timelock`). */
export default buildModule("Timelock", (m) => addTimelock(m));
