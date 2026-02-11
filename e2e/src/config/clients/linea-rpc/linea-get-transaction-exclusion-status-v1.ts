import type { Client, Hex, BlockNumber, Address, Hash, Transport, Chain, Account } from "viem";

export type GetTransactionExclusionStatusV1Parameters = {
  txHash: Hash;
};

export type GetTransactionExclusionStatusV1ReturnType = {
  txHash: Hash;
  from: Address;
  nonce: Hex;
  txRejectionStage: "SEQUENCER" | "RPC" | "P2P";
  reasonMessage: string;
  blockNumber: BlockNumber<Hex>;
  timestamp: string;
};

export type GetTransactionExclusionStatusV1Rpc = {
  Method: "linea_getTransactionExclusionStatusV1";
  Parameters: [GetTransactionExclusionStatusV1Parameters["txHash"]];
  ReturnType: GetTransactionExclusionStatusV1ReturnType;
};

export async function getTransactionExclusionStatusV1(
  client: Client<Transport, Chain | undefined, Account | undefined, [GetTransactionExclusionStatusV1Rpc]>,
  params: GetTransactionExclusionStatusV1Parameters,
): Promise<GetTransactionExclusionStatusV1ReturnType> {
  return client.request({
    method: "linea_getTransactionExclusionStatusV1",
    params: [params.txHash],
  });
}
