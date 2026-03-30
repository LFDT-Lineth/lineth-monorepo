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

function requireEnv(name: string): string {
  const value = process.env[name];
  if (!value) {
    throw new Error(`Missing ${name} environment variable`);
  }
  return value;
}

async function checkDelegation(
  provider: ethers.Provider,
  address: string,
): Promise<{ isDelegated: boolean; implementationAddress?: string }> {
  const code = await provider.getCode(address);

  if (code === "0x") {
    console.log(`No delegation found for ${address}`);
    return { isDelegated: false };
  }

  // EIP-7702 delegation code starts with 0xef0100
  if (code.startsWith("0xef0100")) {
    const delegatedAddress = "0x" + code.slice(8);
    console.log(`Delegation found for ${address} -> ${delegatedAddress}`);
    return { isDelegated: true, implementationAddress: delegatedAddress };
  }

  console.log(`Address has code but not EIP-7702 delegation: ${code}`);
  return { isDelegated: false };
}

async function getAccountInfo(
  provider: ethers.Provider,
  address: string,
): Promise<{ address: string; balance: string; nonce: number }> {
  const balance = await provider.getBalance(address);
  const nonce = await provider.getTransactionCount(address);
  return { address, balance: ethers.formatEther(balance), nonce };
}

// Signs an EIP-7702 authorization tuple.
// nonceOffset: +1 for self-sponsored (sender nonce incremented before auth processing), +0 for sponsored (separate authority)
async function createAuthorization(
  signer: ethers.Wallet,
  provider: ethers.Provider,
  targetContractAddress: string,
  nonceOffset: number,
): Promise<ethers.Authorization> {
  const chainId = (await provider.getNetwork()).chainId;
  const currentNonce = await provider.getTransactionCount(signer.address);
  const authNonce = currentNonce + nonceOffset;

  const authorization = await signer.authorize({
    address: targetContractAddress,
    nonce: authNonce,
    chainId,
  });
  console.log(`Authorization created for ${signer.address} with nonce: ${authorization.nonce}`);
  return authorization;
}

// Estimates gas fees, branching on whether the chain is Linea or not.
async function estimateGasFees(
  provider: ethers.Provider,
  rpcUrl: string,
  from: string,
  to?: string,
  data: string = "0x",
): Promise<{ maxFeePerGas?: bigint; maxPriorityFeePerGas?: bigint; gasPrice?: bigint }> {
  const chainId = Number((await provider.getNetwork()).chainId);
  if (isLineaChainId(chainId)) {
    const client = new LineaEstimateGasClient(new URL(rpcUrl), from);
    const { gasLimit, ...fees } = await client.lineaEstimateGas(to, data);
    void gasLimit;
    return fees;
  }
  return get1559Fees(provider);
}

export {
  get1559Fees,
  isLineaChainId,
  LineaEstimateGasClient,
  requireEnv,
  checkDelegation,
  getAccountInfo,
  createAuthorization,
  estimateGasFees,
};
