import { ethers } from "hardhat";
import { DeployFunction } from "hardhat-deploy/types";
import { deployFromFactory } from "../scripts/hardhat/utils";
import { tryVerifyContract, getRequiredEnvVar, LogContractDeployment } from "../common/helpers";

const func: DeployFunction = async function () {
  const contractName = getRequiredEnvVar("VERIFIER_CONTRACT_NAME");
  const verifierIndex = getRequiredEnvVar("VERIFIER_PROOF_TYPE");

  const provider = ethers.provider;

  const contract = await deployFromFactory(contractName, provider);

  await LogContractDeployment(contractName, contract);
  const contractAddress = await contract.getAddress();

  process.env.PLONKVERIFIER_ADDRESS = contractAddress;

  const setVerifierAddress = ethers.concat([
    "0xc2116974",
    ethers.AbiCoder.defaultAbiCoder().encode(["address", "uint256"], [contractAddress, verifierIndex]),
  ]);

  console.log("setVerifierAddress calldata:", setVerifierAddress);

  await tryVerifyContract(contractAddress);
};
export default func;
func.tags = ["PlonkVerifier"];
