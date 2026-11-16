import { buildModule } from "@nomicfoundation/hardhat-ignition/modules";

/** Parameters: `timelockMinDelay`, `timelockProposers`, `timelockExecutors`, `timelockAdmin`. */

export default buildModule("Timelock", (m) => {
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
});
