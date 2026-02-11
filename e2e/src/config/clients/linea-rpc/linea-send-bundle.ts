import type { Client, Hash, Transport, Chain, Account } from "viem";

export type LineaSendBundleParameters = {
  txs: Hash[];
  replacementUUID: string;
  blockNumber: string;
};

export type LineaSendBundleReturnType = {
  bundleHash: Hash;
};

export type LineaSendBundleRpc = {
  Method: "linea_sendBundle";
  Parameters: [LineaSendBundleParameters];
  ReturnType: LineaSendBundleReturnType;
};

export async function lineaSendBundle(
  client: Client<Transport, Chain | undefined, Account | undefined, [LineaSendBundleRpc]>,
  params: LineaSendBundleParameters,
): Promise<LineaSendBundleReturnType> {
  return client.request({
    method: "linea_sendBundle",
    params: [params],
  });
}
