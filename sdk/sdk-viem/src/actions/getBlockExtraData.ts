import { parseBlockExtraData } from "@consensys/linea-sdk-core";
import { Account, BlockTag, Chain, Client, GetBlockErrorType, GetBlockParameters, Prettify, Transport } from "viem";
import { getBlock } from "viem/actions";

export type GetBlockExtraDataReturnType = Prettify<{
  version: number;
  fixedCost: number;
  variableCost: number;
  ethGasPrice: number;
}>;

export type GetBlockExtraDataParameters<blockTag extends BlockTag = "latest"> = Omit<
  GetBlockParameters<false, blockTag>,
  "includeTransactions"
>;

export type GetBlockExtraDataErrorType = GetBlockErrorType;

/**
 * Returns fomatted linea block extra data.
 *
 * @returns Formatted linea block extra data. {@link GetBlockExtraDataReturnType}
 * @param client - Client to use
 * @param args - {@link GetBlockExtraDataParameters}
 *
 * @example
 * import { createPublicClient, http } from 'viem'
 * import { linea } from 'viem/chains'
 * import { getBlockExtraData } from '@consensys/linea-sdk-viem'
 *
 * const client = createPublicClient({
 *   chain: linea,
 *   transport: http(),
 * });
 *
 * const blockExtraData = await getBlockExtraData(client, {
 *   blockTag: 'latest',
 * });
 */
export async function getBlockExtraData<
  chain extends Chain | undefined,
  account extends Account | undefined,
  blockTag extends BlockTag = "latest",
>(
  client: Client<Transport, chain, account>,
  parameters: GetBlockExtraDataParameters<blockTag>,
): Promise<GetBlockExtraDataReturnType> {
  const block = await getBlock(client, parameters as GetBlockParameters<false, blockTag>);

  return parseBlockExtraData(block.extraData);
}
