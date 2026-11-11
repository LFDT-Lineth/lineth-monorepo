/**
 * Former `13_deploy_EIP2935SystemContract.ts` / `16_deploy_EIP4788SystemContract.ts`: broadcast the presigned
 * (Nick's method, pre-EIP-155) deployment transaction from the EIP. No local signer is involved.
 * Prerequisite: fund the predetermined sender address with enough ETH to cover the deployment cost.
 *
 *   hardhat deploy:eip2935-system-contract --network custom
 *   hardhat deploy:eip4788-system-contract --network custom
 *
 * Against anvil (`anvil --hardfork london`):
 *   CUSTOM_RPC_URL=http://127.0.0.1:8545 pnpm exec hardhat deploy:eip2935-system-contract --network custom
 */
import { getAddress, serializeTransaction } from "viem";

import type { HardhatRuntimeEnvironment } from "hardhat/types/hre";
import type { NetworkConnection } from "hardhat/types/network";
import type { Address, Hex } from "viem";

export type PresignedSystemContract = {
  contractName: string;
  eipLabel: string;
  expectedAddress: Address;
  predeterminedSenderAddress: Address;
  data: Hex;
  r: Hex;
  s: Hex;
};

/** EIP-2935 Historical Block Hashes - https://github.com/ethereum/EIPs/blob/master/EIPS/eip-2935.md#deployment */
export const EIP2935_SYSTEM_CONTRACT: PresignedSystemContract = {
  contractName: "EIP2935SystemContract",
  eipLabel: "EIP-2935",
  expectedAddress: "0x0000F90827F1C53a10cb7A02335B175320002935",
  predeterminedSenderAddress: "0x3462413Af4609098e1E27A490f554f260213D685",
  data: "0x60538060095f395ff33373fffffffffffffffffffffffffffffffffffffffe14604657602036036042575f35600143038111604257611fff81430311604257611fff9006545f5260205ff35b5f5ffd5b5f35611fff60014303065500",
  r: "0x539",
  s: "0xaa12693182426612186309f02cfe8a80a0000",
};

/** EIP-4788 Beacon block root - https://github.com/ethereum/EIPs/blob/master/EIPS/eip-4788.md#deployment */
export const EIP4788_SYSTEM_CONTRACT: PresignedSystemContract = {
  contractName: "EIP4788SystemContract",
  eipLabel: "EIP-4788",
  expectedAddress: "0x000F3df6D732807Ef1319fB7B8bB8522d0Beac02",
  predeterminedSenderAddress: "0x0B799C86a49DEeb90402691F1041aa3AF2d3C875",
  data: "0x60618060095f395ff33373fffffffffffffffffffffffffffffffffffffffe14604d57602036146024575f5ffd5b5f35801560495762001fff810690815414603c575f5ffd5b62001fff01545f5260205ff35b5f5ffd5b62001fff42064281555f359062001fff015500",
  r: "0x539",
  s: "0x1b9b6eb1f0",
};

// Shared by both EIPs' deployment transactions.
const DEPLOYMENT_NONCE = 0;
const DEPLOYMENT_GAS = 0x3d090n;
const DEPLOYMENT_GAS_PRICE = 0xe8d4a51000n;
const DEPLOYMENT_V = 0x1bn;

/** The signed legacy deployment transaction, as `ethers.Transaction.from(...)` + signature serialized it. */
export function serializePresignedDeployment(systemContract: PresignedSystemContract): Hex {
  return serializeTransaction(
    {
      type: "legacy",
      nonce: DEPLOYMENT_NONCE,
      gas: DEPLOYMENT_GAS,
      gasPrice: DEPLOYMENT_GAS_PRICE,
      value: 0n,
      data: systemContract.data,
    },
    { r: systemContract.r, s: systemContract.s, v: DEPLOYMENT_V },
  );
}

/** Broadcasts the presigned deployment unless code already exists at the expected address. Failures fail the task. */
export async function deployPresignedSystemContract(
  connection: NetworkConnection,
  systemContract: PresignedSystemContract,
): Promise<void> {
  const { contractName, expectedAddress } = systemContract;
  const publicClient = await connection.viem.getPublicClient();

  // Check if the contract is already deployed at the expected address
  const code = await publicClient.getCode({ address: expectedAddress });
  if (code !== undefined && code !== "0x") {
    console.log(`${systemContract.eipLabel} system contract already deployed at ${expectedAddress}`);
    return;
  }

  try {
    // Estimate gas to check if the transaction would revert before broadcasting
    console.log("Simulating deployment tx with eth_estimageGas...");
    const estimatedGas = await publicClient.estimateGas({
      account: systemContract.predeterminedSenderAddress,
      data: systemContract.data,
      value: 0n,
      gas: DEPLOYMENT_GAS,
      gasPrice: DEPLOYMENT_GAS_PRICE,
      nonce: DEPLOYMENT_NONCE,
    });
    console.log(`Deployment tx simulated successfully, estimatedGas=${estimatedGas}`);

    console.log(`Broadcasting deployment tx...`);
    const hash = await publicClient.sendRawTransaction({
      serializedTransaction: serializePresignedDeployment(systemContract),
    });
    console.log(`Transaction=${hash} broadcasted`);
    console.log("Waiting for transaction receipt...");
    const receipt = await publicClient.waitForTransactionReceipt({ hash });

    const deployedAddress = receipt.contractAddress ? getAddress(receipt.contractAddress) : receipt.contractAddress;
    console.log(
      `contract=${contractName} deployed: address=${deployedAddress} blockNumber=${receipt.blockNumber} chainId=${await publicClient.getChainId()}`,
    );

    if (deployedAddress !== expectedAddress) {
      throw new Error(`Contract deployed to ${receipt.contractAddress}, expected ${expectedAddress}`);
    }
  } catch (error) {
    throw new Error(
      `Error during contract=${contractName} deployment: ${error instanceof Error ? error.message : String(error)}`,
      { cause: error },
    );
  }
}

export async function eip2935SystemContractAction(_taskArgs: unknown, hre: HardhatRuntimeEnvironment): Promise<void> {
  await deployPresignedSystemContract(await hre.network.getOrCreate(), EIP2935_SYSTEM_CONTRACT);
}

export async function eip4788SystemContractAction(_taskArgs: unknown, hre: HardhatRuntimeEnvironment): Promise<void> {
  await deployPresignedSystemContract(await hre.network.getOrCreate(), EIP4788_SYSTEM_CONTRACT);
}
