import { ethers, getChainId } from "hardhat";
import { DeployFunction } from "hardhat-deploy/types";

// Deploy EIP-4788 Historical Block Hashes system contract - https://github.com/ethereum/EIPs/blob/master/EIPS/eip-4788.md
// Prerequisite - Fund the predetermined sender address with enough ETH to cover the deployment cost

// Run deploy script against anvil `anvil --hardfork london`
// DEPLOYER_PRIVATE_KEY=ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80 CUSTOM_RPC_URL=http://127.0.0.1:8545 npx hardhat deploy --network custom --tags EIP4788SystemContract

// Run deploy script against local stack `make start-env-with-tracing-v2-ci CLEAN_PREVIOUS_ENV=true`
// DEPLOYER_PRIVATE_KEY=1dd171cec7e2995408b5513004e8207fe88d6820aeff0d82463b3e41df251aae CUSTOM_RPC_URL=http://127.0.0.1:9045 npx hardhat deploy --network custom --tags EIP4788SystemContract

const func: DeployFunction = async function () {
  const contractName = "EIP4788SystemContract";
  const expectedAddress = "0x000F3df6D732807Ef1319fB7B8bB8522d0Beac02";

  // Check if the contract is already deployed at the expected address
  const code = await ethers.provider.getCode(expectedAddress);
  if (code !== "0x") {
    console.log(`EIP-4788 system contract already deployed at ${expectedAddress}`);
    return;
  }

  // EIP-4788 deployment transaction data - https://github.com/ethereum/EIPs/blob/master/EIPS/eip-4788.md#deployment
  const deploymentTx = {
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
  };

  // The predetermined sender address from EIP-4788
  const predeterminedSenderAddress = "0x0B799C86a49DEeb90402691F1041aa3AF2d3C875";

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
func.tags = ["EIP4788SystemContract"];
