export type Message = {
  messageSender: string;
  destination: string;
  fee: bigint;
  value: bigint;
  messageNonce: bigint;
  calldata: string;
  messageHash: string;
};
