import { task } from "hardhat/config";
import { getTaskCliOrEnvValue } from "../../../common/helpers/environmentHelper";

/*
    *******************************************************************************************
    1. Deploy the verifier and get the address
    2. Run this script matching the correct PROOF_TYPE
    *******************************************************************************************

    *******************************************************************************************
    DEPLOYER_PRIVATE_KEY=<key> \
    INFURA_API_KEY=<key> \
    npx hardhat setVerifierAddress \
    --verifier-proof-type <uint256> \
    --proxy-address <address> \
    --verifier-address <address> \
    --verifier-name <string> \
    --network sepolia
    *******************************************************************************************
*/

task("setVerifierAddress", "Sets the verifier address on a Message Service contract")
  .addOptionalParam("verifierProofType")
  .addOptionalParam("proxyAddress")
  .addOptionalParam("verifierAddress")
  .addOptionalParam("verifierName")
  .setAction(async (taskArgs, hre) => {
    const ethers = hre.ethers;

    const { deployments, getNamedAccounts } = hre;
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    const { deployer } = await getNamedAccounts();
    const { get } = deployments;

    const proofType = getTaskCliOrEnvValue(taskArgs, "verifierProofType", "VERIFIER_PROOF_TYPE");
    let LineaRollupAddress = getTaskCliOrEnvValue(taskArgs, "proxyAddress", "LINEA_ROLLUP_ADDRESS");
    const verifierName = getTaskCliOrEnvValue(taskArgs, "verifierContractName", "VERIFIER_CONTRACT_NAME");

    if (LineaRollupAddress === undefined) {
      LineaRollupAddress = (await get("LineaRollup")).address;
    }

    let verifierAddress = getTaskCliOrEnvValue(taskArgs, "verifierAddress", "VERIFIER_ADDRESS");
    if (verifierAddress === undefined) {
      if (verifierName === undefined) {
        throw "Please specify a verifier name e.g. --verifier-contract-name PlonkVerifierDev";
      }
      verifierAddress = (await get(verifierName)).address;
    }

    if (!proofType) {
      throw "Please specify a verifierProofType";
    }

    const LineaRollup = await ethers.getContractAt("LineaRollup", LineaRollupAddress);

    console.log(`Setting verifier address ${verifierAddress} of type ${proofType}`);
    const tx = await LineaRollup.setVerifierAddress(verifierAddress, proofType);

    console.log("Waiting for transaction to process");
    await tx.wait();

    const checkVerifierIsSet = await LineaRollup.verifiers(proofType);
    console.log(`Linea Rollup implementation added ${checkVerifierIsSet} as new verifier`);
  });
