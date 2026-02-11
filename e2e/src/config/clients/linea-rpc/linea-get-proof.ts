import type { Client, Hex, Transport, Chain, Account } from "viem";

export type LineaGetProofParameters = {
  address: string;
  storageKeys: string[];
  blockParameter: string;
};

export type LineaGetProofReturnType = {
  accountProof: {
    key: Hex;
    leafIndex: number;
    proof: {
      value: Hex;
      proofRelatedNodes: Hex[];
    };
  };
  storageProofs: {
    key: Hex;
    leafIndex: number;
    proof: {
      value: Hex;
      proofRelatedNodes: Hex[];
    };
  }[];
};

export type LineaGetProofRpc = {
  Method: "linea_getProof";
  Parameters: [
    LineaGetProofParameters["address"],
    LineaGetProofParameters["storageKeys"],
    LineaGetProofParameters["blockParameter"],
  ];
  ReturnType: LineaGetProofReturnType;
};

export async function lineaGetProof(
  client: Client<Transport, Chain | undefined, Account | undefined, [LineaGetProofRpc]>,
  params: LineaGetProofParameters,
): Promise<LineaGetProofReturnType> {
  const { address, storageKeys = [], blockParameter = "latest" } = params;

  return client.request({
    method: "linea_getProof",
    params: [address, storageKeys, blockParameter],
  });
}
