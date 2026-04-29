import type { Client, Hash, Transport, Chain, Account } from "viem";

export type ForcedTransactionInclusionStatus = {
  inclusionResult: string;
  transactionHash: string;
};

export type LineaGetForcedTransactionInclusionStatusRpc = {
  Method: "linea_getForcedTransactionInclusionStatus";
  Parameters: [Hash];
  ReturnType: ForcedTransactionInclusionStatus | null;
};

export async function lineaGetForcedTransactionInclusionStatus(
  client: Client<Transport, Chain | undefined, Account | undefined, [LineaGetForcedTransactionInclusionStatusRpc]>,
  transactionHash: Hash,
): Promise<ForcedTransactionInclusionStatus | null> {
  return client.request({
    method: "linea_getForcedTransactionInclusionStatus",
    params: [transactionHash],
  });
}
