import { Account, Chain, Client, Transport } from "viem";
import { estimateGas, EstimateGasParameters } from "viem/linea";

export async function estimateLineaGas<
  chain extends Chain | undefined = Chain | undefined,
  account extends Account | undefined = Account | undefined,
>(client: Client<Transport, chain, account>, params: EstimateGasParameters<chain>) {
  const BASE_FEE_NUMERATOR = 135n;
  const PRIORITY_FEE_NUMERATOR = 105n;
  const DENOMINATOR = 100n;
  const result = await estimateGas(client, params);

  const baseFeePerGas = (result.baseFeePerGas * BASE_FEE_NUMERATOR) / DENOMINATOR;
  const maxPriorityFeePerGas = (result.priorityFeePerGas * PRIORITY_FEE_NUMERATOR) / DENOMINATOR;

  return {
    maxFeePerGas: baseFeePerGas + maxPriorityFeePerGas,
    maxPriorityFeePerGas,
    gasLimit: result.gasLimit,
  };
}
