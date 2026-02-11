import type { Client, Hex, Transport, Chain, Account } from "viem";

export type SaveRejectedTransactionV1Parameters = {
  txRejectionStage: "SEQUENCER" | "RPC" | "P2P";
  timestamp: string;
  blockNumber: number | null;
  transactionRLP: Hex;
  reasonMessage: string;
  overflows: { module: string; count: number; limit: number }[];
};

export type SaveRejectedTransactionV1ReturnType = boolean;

export type SaveRejectedTransactionV1Rpc = {
  Method: "linea_saveRejectedTransactionV1";
  Parameters: [SaveRejectedTransactionV1Parameters];
  ReturnType: SaveRejectedTransactionV1ReturnType;
};

export async function saveRejectedTransactionV1(
  client: Client<Transport, Chain | undefined, Account | undefined, [SaveRejectedTransactionV1Rpc]>,
  params: SaveRejectedTransactionV1Parameters,
): Promise<SaveRejectedTransactionV1ReturnType> {
  return client.request({
    method: "linea_saveRejectedTransactionV1",
    params: [params],
  });
}
