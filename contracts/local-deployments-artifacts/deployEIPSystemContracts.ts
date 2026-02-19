import { ethers } from "ethers";
import * as dotenv from "dotenv";

dotenv.config();

interface EIPContractConfig {
  contractName: string;
  expectedAddress: string;
  predeterminedSenderAddress: string;
  deploymentTx: {
    type: number;
    nonce: number;
    to: null;
    gas: number;
    gasPrice: number;
    value: number;
    data: string;
    v: number;
    r: string;
    s: string;
  };
}

const EIP_CONTRACTS: Record<string, EIPContractConfig> = {
  EIP2935: {
    contractName: "EIP2935SystemContract",
    expectedAddress: "0x0000F90827F1C53a10cb7A02335B175320002935",
    predeterminedSenderAddress: "0x3462413Af4609098e1E27A490f554f260213D685",
    deploymentTx: {
      type: 0x0,
      nonce: 0x0,
      to: null,
      gas: 0x3d090,
      gasPrice: 0xe8d4a51000,
      value: 0x0,
      data: "0x60538060095f395ff33373fffffffffffffffffffffffffffffffffffffffe14604657602036036042575f35600143038111604257611fff81430311604257611fff9006545f5260205ff35b5f5ffd5b5f35611fff60014303065500",
      v: 0x1b,
      r: "0x539",
      s: "0xaa12693182426612186309f02cfe8a80a0000",
    },
  },
  EIP4788: {
    contractName: "EIP4788SystemContract",
    expectedAddress: "0x000F3df6D732807Ef1319fB7B8bB8522d0Beac02",
    predeterminedSenderAddress: "0x0B799C86a49DEeb90402691F1041aa3AF2d3C875",
    deploymentTx: {
      type: 0x0,
      nonce: 0x0,
      to: null,
      gas: 0x3d090,
      gasPrice: 0xe8d4a51000,
      value: 0x0,
      data: "0x60618060095f395ff33373fffffffffffffffffffffffffffffffffffffffe14604d57602036146024575f5ffd5b5f35801560495762001fff810690815414603c575f5ffd5b62001fff01545f5260205ff35b5f5ffd5b62001fff42064281555f359062001fff015500",
      v: 0x1b,
      r: "0x539",
      s: "0x1b9b6eb1f0",
    },
  },
};

async function deployEIPSystemContract(
  provider: ethers.JsonRpcProvider,
  wallet: ethers.Wallet,
  walletNonce: number,
  config: EIPContractConfig,
): Promise<void> {
  try {
    // Prefund expected sender
    const prefundTx = await wallet.sendTransaction({
      to: config.predeterminedSenderAddress,
      value: BigInt(config.deploymentTx.gas) * BigInt(config.deploymentTx.gasPrice),
      nonce: walletNonce,
    });
    await prefundTx.wait();
    console.log(`Successfully prefunded predetermined sender for ${config.contractName}`);
  } catch (error) {
    console.error(
      `❌ Error during prefunding predetermined sender for ${config.contractName}: ${(error as Error).message}`,
    );
    throw error;
  }

  // Check if the contract is already deployed at the expected address
  const code = await provider.getCode(config.expectedAddress);
  if (code !== "0x") {
    console.log(`${config.contractName} already deployed at ${config.expectedAddress}`);
    return;
  }

  // Create and serialize the raw transaction with signature
  const rawTx = ethers.Transaction.from({
    type: config.deploymentTx.type,
    nonce: config.deploymentTx.nonce,
    to: config.deploymentTx.to,
    gasLimit: config.deploymentTx.gas,
    gasPrice: config.deploymentTx.gasPrice,
    value: config.deploymentTx.value,
    data: config.deploymentTx.data,
  });

  rawTx.signature = ethers.Signature.from({
    v: config.deploymentTx.v,
    r: config.deploymentTx.r,
    s: config.deploymentTx.s,
  });

  try {
    const estimatedGas = await provider.estimateGas({
      from: config.predeterminedSenderAddress,
      to: config.deploymentTx.to,
      value: config.deploymentTx.value,
      data: config.deploymentTx.data,
      gasPrice: config.deploymentTx.gasPrice,
      gasLimit: config.deploymentTx.gas,
      type: config.deploymentTx.type,
      nonce: config.deploymentTx.nonce,
    });
    console.log(`${config.contractName} deployment tx simulated successfully, estimatedGas=${estimatedGas}`);
  } catch (error) {
    console.error(`❌ Error during ${config.contractName} estimateGas: ${(error as Error).message}`);
    throw error;
  }

  try {
    const tx = await provider.broadcastTransaction(rawTx.serialized);
    const receipt = await tx.wait();
    const chainId = (await provider.getNetwork()).chainId;
    console.log(
      `contract=${config.contractName} deployed: address=${receipt?.contractAddress} blockNumber=${receipt?.blockNumber} chainId=${chainId}`,
    );

    if (receipt?.contractAddress !== config.expectedAddress) {
      throw new Error(`Contract deployed to ${receipt?.contractAddress}, expected ${config.expectedAddress}`);
    }
  } catch (error) {
    console.error(`❌ Error during ${config.contractName} deployment: ${(error as Error).message}`);
    throw error;
  }
}

async function main() {
  console.log("Starting deployment of EIP system contracts...");

  if (!process.env.RPC_URL) {
    throw new Error("RPC_URL environment variable is required");
  }

  const provider = new ethers.JsonRpcProvider(process.env.RPC_URL);
  const wallet = new ethers.Wallet(process.env.DEPLOYER_PRIVATE_KEY!, provider);

  const contractNames = ["EIP2935", "EIP4788"] as const;

  let walletNonce: number;
  if (!process.env.L2_NONCE) {
    walletNonce = await wallet.getNonce();
  } else {
    walletNonce = parseInt(process.env.L2_NONCE);
  }

  for (const [index, contractName] of contractNames.entries()) {
    const config = EIP_CONTRACTS[contractName];
    await deployEIPSystemContract(provider, wallet, walletNonce + index, config);
  }
}

main().catch((error) => {
  console.error("Deployment failed:", error);
  process.exit(1);
});
