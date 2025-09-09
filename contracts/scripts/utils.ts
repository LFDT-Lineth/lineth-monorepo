import { SupportedChainIds } from "contracts/common/supportedNetworks";
import { ethers } from "ethers";

/**
 * @param provider ethers JsonRpcProvider or HardhatEthersHelpers provider instance
 * @returns {Promise<{maxPriorityFeePerGas: *, maxFeePerGas: *}>}
 */
async function get1559Fees(
  provider: ethers.Provider,
): Promise<{ maxPriorityFeePerGas?: bigint; maxFeePerGas?: bigint; gasPrice?: bigint }> {
  const { maxPriorityFeePerGas, maxFeePerGas, gasPrice } = await provider.getFeeData();
  return {
    ...(maxPriorityFeePerGas ? { maxPriorityFeePerGas } : {}),
    ...(maxFeePerGas ? { maxFeePerGas } : {}),
    ...(gasPrice ? { gasPrice } : {}),
  };
}

function isLineaChainId(chainId: number): boolean {
  const lineaChainIds = new Set<number>([
    SupportedChainIds.LINEA_DEVNET,
    SupportedChainIds.LINEA_SEPOLIA,
    SupportedChainIds.LINEA_TESTNET,
    SupportedChainIds.LINEA,
  ]);
  return lineaChainIds.has(chainId);
}

class LineaEstimateGasClient {
  private endpoint: URL;
  private BASE_FEE_MULTIPLIER = 1.35;
  private PRIORITY_FEE_MULTIPLIER = 1.05;
  private fromAddress: string;

  public constructor(endpoint: URL, fromAddress: string) {
    this.endpoint = endpoint;
    this.fromAddress = fromAddress;
  }

  public async lineaEstimateGas(
    to?: string,
    data: string = "0x",
    value: string = "0x0",
  ): Promise<{ maxFeePerGas: bigint; maxPriorityFeePerGas: bigint; gasLimit: bigint }> {
    const from = this.fromAddress;
    const request = {
      method: "post",
      body: JSON.stringify({
        jsonrpc: "2.0",
        method: "linea_estimateGas",
        params: [
          {
            from,
            to,
            data,
            value,
          },
        ],
        id: Math.floor(Math.random() * 1001),
      }),
    };
    const response = await fetch(this.endpoint, request);
    const responseJson = await response.json();

    const baseFeePerGas = this.getValueFromMultiplier(
      BigInt(responseJson.result.baseFeePerGas),
      this.BASE_FEE_MULTIPLIER,
    );
    const maxPriorityFeePerGas = this.getValueFromMultiplier(
      BigInt(responseJson.result.priorityFeePerGas),
      this.PRIORITY_FEE_MULTIPLIER,
    );

    return {
      maxFeePerGas: baseFeePerGas + maxPriorityFeePerGas,
      maxPriorityFeePerGas,
      gasLimit: BigInt(responseJson.result.gasLimit),
    };
  }

  private getValueFromMultiplier(value: bigint, multiplier: number): bigint {
    return (value * BigInt(multiplier * 100)) / 100n;
  }
}

export { get1559Fees, isLineaChainId, LineaEstimateGasClient };
