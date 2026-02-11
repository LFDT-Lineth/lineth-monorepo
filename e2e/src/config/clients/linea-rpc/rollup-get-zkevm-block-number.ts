import type { Client, Transport, Chain, Account, Hex } from "viem";

export type GetZkEVMBlockNumberReturnType = bigint;

export type GetZkEVMBlockNumberRpc = {
  Method: "rollup_getZkEVMBlockNumber";
  Parameters: [];
  ReturnType: Hex;
};

export async function getZkEVMBlockNumber(
  client: Client<Transport, Chain | undefined, Account | undefined, [GetZkEVMBlockNumberRpc]>,
): Promise<GetZkEVMBlockNumberReturnType> {
  const blockNumber = await client.request({
    method: "rollup_getZkEVMBlockNumber",
    params: [],
  });

  return BigInt(blockNumber);
}
