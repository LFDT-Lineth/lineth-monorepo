import type { Client, Transport, Chain, Account } from "viem";

export type LineaCancelBundleParameters = {
  replacementUUID: string;
};

export type LineaCancelBundleReturnType = boolean;

export type LineaCancelBundleRpc = {
  Method: "linea_cancelBundle";
  Parameters: [LineaCancelBundleParameters["replacementUUID"]];
  ReturnType: LineaCancelBundleReturnType;
};

export async function lineaCancelBundle(
  client: Client<Transport, Chain | undefined, Account | undefined, [LineaCancelBundleRpc]>,
  params: LineaCancelBundleParameters,
): Promise<LineaCancelBundleReturnType> {
  return client.request({
    method: "linea_cancelBundle",
    params: [params.replacementUUID],
  });
}
