import { buildModule } from "@nomicfoundation/hardhat-ignition/modules";

/** Parameters: `payees`, `shares`. */
export default buildModule("PaymentSplitterWrapper", (m) => {
  const paymentSplitterWrapper = m.contract(
    "PaymentSplitterWrapper",
    [m.getParameter("payees"), m.getParameter("shares")],
    { id: "PaymentSplitterWrapper" },
  );
  return { paymentSplitterWrapper };
});
