import { OnChainMessageStatus } from "@consensys/linea-sdk";

type StatusTextProps = {
  status: OnChainMessageStatus;
};

const getLabel = (status: OnChainMessageStatus): string => {
  switch (status) {
    case OnChainMessageStatus.CLAIMABLE:
      return "Ready to claim";
    case OnChainMessageStatus.UNKNOWN:
      return "Pending";
    case OnChainMessageStatus.CLAIMED:
      return "Completed";
    default:
      throw new Error(`Incorrect transaction status: ${status}`);
  }
};

export default function StatusText({ status }: StatusTextProps) {
  return <span>{getLabel(status)}</span>;
}
