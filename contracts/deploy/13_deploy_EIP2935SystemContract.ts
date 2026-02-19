import { ethers, getChainId } from "hardhat";
import { DeployFunction } from "hardhat-deploy/types";

// Deploy EIP-2935 Historical Block Hashes system contract - https://github.com/ethereum/EIPs/blob/master/EIPS/eip-2935.md
// Prerequisite - Fund the predetermined sender address with enough ETH to cover the deployment cost

// Run deploy script against anvil `anvil --hardfork london`
// DEPLOYER_PRIVATE_KEY=ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80 CUSTOM_RPC_URL=http://127.0.0.1:8545 npx hardhat deploy --network custom --tags EIP2935SystemContract

// Run deploy script against local stack `make start-env-with-tracing-v2-ci CLEAN_PREVIOUS_ENV=true`
// DEPLOYER_PRIVATE_KEY=1dd171cec7e2995408b5513004e8207fe88d6820aeff0d82463b3e41df251aae CUSTOM_RPC_URL=http://127.0.0.1:9045 npx hardhat deploy --network custom --tags EIP2935SystemContract

const func: DeployFunction = async function () {
  const contractName = "EIP2935SystemContract";
  const expectedAddress = "0x0000F90827F1C53a10cb7A02335B175320002935";

  // Check if the contract is already deployed at the expected address
  const code = await ethers.provider.getCode(expectedAddress);
  if (code !== "0x") {
    console.log(`EIP-2935 system contract already deployed at ${expectedAddress}`);
    return;
  }

  // EIP-2935 deployment transaction data - https://github.com/ethereum/EIPs/blob/master/EIPS/eip-2935.md#deployment
  const deploymentTx = {
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
  };

  // The predetermined sender address from EIP-2935
  const predeterminedSenderAddress = "0x3462413Af4609098e1E27A490f554f260213D685";

  // Serialize the raw transaction with signature
  const rawTx = ethers.Transaction.from({
    type: deploymentTx.type,
    nonce: deploymentTx.nonce,
    to: deploymentTx.to,
    gasLimit: deploymentTx.gas,
    gasPrice: deploymentTx.gasPrice,
    value: deploymentTx.value,
    data: deploymentTx.data,
  });

  rawTx.signature = ethers.Signature.from({
    v: deploymentTx.v,
    r: deploymentTx.r,
    s: deploymentTx.s,
  });

  try {
    // Estimate gas to check if the transaction would revert before broadcasting
    console.log("Simulating deployment tx with eth_estimageGas...");
    const estimatedGas = await ethers.provider.estimateGas({
      from: predeterminedSenderAddress,
      to: deploymentTx.to,
      value: deploymentTx.value,
      data: deploymentTx.data,
      gasPrice: deploymentTx.gasPrice,
      gasLimit: deploymentTx.gas,
      type: deploymentTx.type,
      nonce: deploymentTx.nonce,
    });
    console.log(`Deployment tx simulated successfully, estimatedGas=${estimatedGas}`);

    console.log(`Broadcasting deployment tx...`);
    const tx = await ethers.provider.broadcastTransaction(rawTx.serialized);
    console.log(`Transaction=${tx.hash} broadcasted`);
    console.log("Waiting for transaction receipt...");
    const receipt = await tx.wait();

    console.log(
      `contract=${contractName} deployed: address=${receipt?.contractAddress} blockNumber=${receipt?.blockNumber} chainId=${await getChainId()}`,
    );

    if (receipt?.contractAddress !== expectedAddress) {
      throw new Error(`Contract deployed to ${receipt?.contractAddress}, expected ${expectedAddress}`);
    }
  } catch (error) {
    console.error(`Error during contract=${contractName} deployment: ${(error as Error).message}`);
  }
};
export default func;
func.tags = ["EIP2935SystemContract"];
