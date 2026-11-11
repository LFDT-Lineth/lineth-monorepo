import { buildModule } from "@nomicfoundation/hardhat-ignition/modules";

/**
 * Replaces `22_deploy_ForcedTransactionGateway.ts` (tag `ForcedTransactionGateway`), linked against an existing `Mimc`.
 *
 * Parameters: `mimcLibrary`, `linethRollup`, `destinationChainId`, `l2BlockBuffer`, `maxGasLimit`, `maxInputLengthBuffer`,
 * `securityCouncil`, `addressFilter`, `l2BlockDurationSeconds`, `blockNumberDeadlineBuffer`.
 */
export default buildModule("ForcedTransactionGateway", (m) => {
  const mimc = m.contractAt("Mimc", m.getParameter<string>("mimcLibrary"), { id: "Mimc" });
  const forcedTransactionGateway = m.contract(
    "ForcedTransactionGateway",
    [
      m.getParameter<string>("linethRollup"),
      m.getParameter("destinationChainId"),
      m.getParameter("l2BlockBuffer"),
      m.getParameter("maxGasLimit"),
      m.getParameter("maxInputLengthBuffer"),
      m.getParameter<string>("securityCouncil"),
      m.getParameter<string>("addressFilter"),
      m.getParameter("l2BlockDurationSeconds"),
      m.getParameter("blockNumberDeadlineBuffer"),
    ],
    { id: "ForcedTransactionGateway", libraries: { Mimc: mimc } },
  );
  return { forcedTransactionGateway, mimc };
});
