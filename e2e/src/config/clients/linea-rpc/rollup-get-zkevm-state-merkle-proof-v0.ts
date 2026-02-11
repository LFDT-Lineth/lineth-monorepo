import type { Client, Hash, Transport, Chain, Account } from "viem";

export type RollupGetZkEVMStateMerkleProofV0Parameters = {
  startBlockNumber: number;
  endBlockNumber: number;
  zkStateManagerVersion: string;
};

export type RollupGetZkEVMStateMerkleProofV0ReturnType = {
  zkEndStateRootHash: Hash;
};

export type RollupGetZkEVMStateMerkleProofV0Rpc = {
  Method: "rollup_getZkEVMStateMerkleProofV0";
  Parameters: [RollupGetZkEVMStateMerkleProofV0Parameters];
  ReturnType: RollupGetZkEVMStateMerkleProofV0ReturnType;
};

export async function rollupGetZkEVMStateMerkleProofV0(
  client: Client<Transport, Chain | undefined, Account | undefined, [RollupGetZkEVMStateMerkleProofV0Rpc]>,
  params: RollupGetZkEVMStateMerkleProofV0Parameters,
): Promise<RollupGetZkEVMStateMerkleProofV0ReturnType> {
  return client.request({
    method: "rollup_getZkEVMStateMerkleProofV0",
    params: [params],
  });
}
