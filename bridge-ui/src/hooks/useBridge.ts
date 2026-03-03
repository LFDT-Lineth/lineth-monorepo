import useTransactionSteps from "./transaction-args/useTransactionSteps";
import useTransactionSender from "./useTransactionSender";

const useBridge = () => {
  const transactionArgs = useTransactionSteps();
  const { send, ...txState } = useTransactionSender(transactionArgs?.args);

  return {
    transactionType: transactionArgs?.type,
    adapterId: transactionArgs?.adapterId,
    refetchAllowance: transactionArgs?.refetchAllowance,
    bridge: send,
    ...txState,
  };
};

export default useBridge;
