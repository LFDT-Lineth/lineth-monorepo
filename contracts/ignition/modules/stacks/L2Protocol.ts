import { buildModule } from "@nomicfoundation/hardhat-ignition/modules";

import TimelockModule from "../governance/Timelock.js";
import L2MessageServiceModule from "../messaging/L2MessageService.js";

/**
 * L2MessageService and Timelock. Parameters: `L2MessageService.*`, `Timelock.*`.
 */
export default buildModule("L2Protocol", (m) => {
  const l2MessageService = m.useModule(L2MessageServiceModule);
  const { timelock } = m.useModule(TimelockModule);
  return { ...l2MessageService, timelock };
});
