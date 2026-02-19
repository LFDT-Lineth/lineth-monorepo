import { Authorization, ethers } from "ethers";
import { get1559Fees, isLineaChainId, LineaEstimateGasClient } from "../utils";
import * as dotenv from "dotenv";
import { generateFunctionSelector } from "contracts/common/helpers";

// Prerequisite - Deploy a contract with NON-VIEW initialize() function, e.g. TestEIP7702Delegation
// Use this contract for TARGET_ADDRESS env

// RPC_URL=<> DEPLOYER_PRIVATE_KEY=<> TARGET_ADDRESS=<> npx hardhat run scripts/testEIP7702/sendType4Tx.ts

dotenv.config();

class EIP7702TransactionSender {
  private provider: ethers.Provider;
  private signer: ethers.Wallet;
  private lineaEstimateGasClient: LineaEstimateGasClient;

  constructor(rpcUrl: string, privateKey: string) {
    this.provider = new ethers.JsonRpcProvider(rpcUrl);
    this.signer = new ethers.Wallet(privateKey, this.provider);
    this.lineaEstimateGasClient = new LineaEstimateGasClient(new URL(rpcUrl), this.signer.address);
  }

  // Create Authorization for process.env.DEPLOYER_PRIVATE_KEY
  async createAuthorizationForSelf(targetContractAddress: string): Promise<Authorization> {
    const network = await this.provider.getNetwork();
    const currentChainId = network.chainId;

    const currentNonce = await this.provider.getTransactionCount(this.signer.address);
    // +1 required when same account is `to` and is an authorization signer
    // https://eips.ethereum.org/EIPS/eip-7702 - "The authorization list is processed before the execution portion of the transaction begins, but after the sender’s nonce is incremented."
    const authNonce = currentNonce + 1;

    const authorization = await this.signer.authorize({
      address: targetContractAddress,
      nonce: authNonce,
      chainId: currentChainId,
    });
    console.log("Authorization created with nonce:", authorization.nonce);
    return authorization;
  }

  // Create Authorization for another private key
  async createAuthorizationForPrivateKey(targetContractAddress: string, privateKey: string): Promise<Authorization> {
    const network = await this.provider.getNetwork();
    const currentChainId = network.chainId;

    const signer = new ethers.Wallet(privateKey, this.provider);
    const currentNonce = await this.provider.getTransactionCount(signer.address);
    const authNonce = currentNonce;

    const authorization = await signer.authorize({
      address: targetContractAddress,
      nonce: authNonce,
      chainId: currentChainId,
    });
    console.log("Authorization created with nonce:", authorization.nonce);
    return authorization;
  }

  async sendNonSponsoredTransaction(targetAddress: string) {
    console.log("\n=== TRANSACTION 1: NON-SPONSORED (ETH TRANSFERS) ===");

    // Create authorization with incremented nonce for same-wallet transactions
    const authorization = await this.createAuthorization(targetAddress);

    const ABI = ["function initialize() external"];

    // Create contract instance and execute
    const delegatedContract = new ethers.Contract(this.signer, ABI, this.signer);

    const chainId = (await this.provider.getNetwork()).chainId;
    const { maxPriorityFeePerGas, maxFeePerGas } = isLineaChainId(Number(chainId))
      ? await this.lineaEstimateGasClient.lineaEstimateGas(
          this.signer.address,
          `0x${generateFunctionSelector("initialize()")}`,
        )
      : await get1559Fees(this.provider);

    const txParams = {
      type: 4,
      authorizationList: [authorization],
      gasLimit: 500000n,
      value: 0n,
      maxPriorityFeePerGas: maxPriorityFeePerGas,
      maxFeePerGas: maxFeePerGas,
    };

    const tx = await delegatedContract["initialize()"](txParams);

    console.log("Non-sponsored transaction sent:", tx.hash);

    const receipt = await tx.wait();
    console.log("Receipt for non-sponsored transaction:", receipt);

    return receipt;
  }

  async checkDelegation(address: string): Promise<{ isDelegated: boolean; implementationAddress?: string }> {
    console.log("\n=== CHECKING DELEGATION STATUS ===");
    const code = await this.provider.getCode(address);

    if (code === "0x") {
      console.log(`❌ No delegation found for ${address}`);
      return { isDelegated: false };
    }

    // Check if it's an EIP-7702 delegation (starts with 0xef0100)
    if (code.startsWith("0xef0100")) {
      // Extract the delegated address (remove 0xef0100 prefix)
      const delegatedAddress = "0x" + code.slice(8); // Remove 0xef0100 (8 chars)

      console.log(`✅ Delegation found for ${address}`);
      console.log(`📍 Delegated to: ${delegatedAddress}`);
      console.log(`📝 Full delegation code: ${code}`);

      return { isDelegated: true, implementationAddress: delegatedAddress };
    } else {
      console.log(`❓ Address has code but not EIP-7702 delegation: ${code}`);
      return { isDelegated: false };
    }
  }

  async getSignerInfo(): Promise<{ address: string; balance: string; nonce: number }> {
    const address = this.signer.address;
    const balance = await this.provider.getBalance(address);
    const nonce = await this.provider.getTransactionCount(address);

    return {
      address,
      balance: ethers.formatEther(balance),
      nonce,
    };
  }
}

function requireEnv(name: string): string {
  const envVariable = process.env[name];
  if (!envVariable) {
    throw new Error(`Missing ${name} environment variable`);
  }

  return envVariable;
}

// Example usage
async function main() {
  try {
    const rpcUrl = requireEnv("RPC_URL");
    const privateKey = requireEnv("DEPLOYER_PRIVATE_KEY");
    const targetAddress = requireEnv("TARGET_ADDRESS");

    const sender = new EIP7702TransactionSender(rpcUrl, privateKey);
    const signerInfo = await sender.getSignerInfo();

    // Check if EOA is already delegated
    const delegationStatus = await sender.checkDelegation(signerInfo.address);
    console.log(`EOA delegation status:`, delegationStatus);

    await sender.sendNonSponsoredTransaction(targetAddress);
  } catch (error) {
    console.error("Error:", error);
    process.exit(1);
  }
}

if (require.main === module) {
  main();
}
