import { createPublicClient, http, type PublicClient, rpcSchema, type Transport } from "viem";

import { type Transaction, type Txpool } from "./types.js";

export type ClientType = "geth" | "besu";

type Web3ClientVersionRpc = {
  Method: "web3_clientVersion";
  Parameters: [];
  ReturnType: string;
};

type GethTxpoolContentRpc = {
  Method: "txpool_content";
  Parameters: [];
  ReturnType: Txpool;
};

type BesuPendingTransactionsRpc = {
  Method: "txpool_besuPendingTransactions";
  Parameters: [number];
  ReturnType: Transaction[];
};

type SynctxRpcSchema = [Web3ClientVersionRpc, GethTxpoolContentRpc, BesuPendingTransactionsRpc];

export type SynctxClient = PublicClient<Transport, undefined, undefined, SynctxRpcSchema>;

export type TransactionPool = Txpool | Transaction[];

export const createSynctxClient = (nodeUrl: string): SynctxClient =>
  createPublicClient({
    transport: http(nodeUrl),
    rpcSchema: rpcSchema<SynctxRpcSchema>(),
  });

export const getClientType = async (client: SynctxClient): Promise<ClientType> => {
  const res = await client.request({ method: "web3_clientVersion", params: [] });
  const clientType = res.slice(0, 4).toLowerCase();
  if (!["geth", "besu"].includes(clientType)) {
    throw new Error(`Invalid node client type, must be either geth or besu`);
  }
  return clientType as ClientType;
};

export const getTransactionPool = async (client: SynctxClient, clientType: ClientType): Promise<TransactionPool> => {
  if (clientType === "geth") {
    return client.request({ method: "txpool_content", params: [] });
  }

  return client.request({ method: "txpool_besuPendingTransactions", params: [2000] });
};
