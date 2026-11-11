import { buildModule } from "@nomicfoundation/hardhat-ignition/modules";

/** Replaces `23_deploy_PaymentSplitterWrapper.ts` (tag `PaymentSplitterWrapper`). Parameters: `payees`, `shares`. */
export default buildModule("PaymentSplitterWrapper", (m) => {
  const paymentSplitterWrapper = m.contract(
    "PaymentSplitterWrapper",
    [m.getParameter("payees"), m.getParameter("shares")],
    { id: "PaymentSplitterWrapper" },
  );
  return { paymentSplitterWrapper };
});
