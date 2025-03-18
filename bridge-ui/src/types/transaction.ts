export type TransactionType = "bridge" | "approve" | "claim";

export enum TransactionStatus {
  READY_TO_CLAIM = "READY_TO_CLAIM",
  PENDING = "PENDING",
  COMPLETED = "COMPLETED",
}
