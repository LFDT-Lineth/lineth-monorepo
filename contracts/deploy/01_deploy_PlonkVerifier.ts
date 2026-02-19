import { ethers } from "hardhat";
import { DeployFunction } from "hardhat-deploy/types";
import { deployFromFactory, deployFromFactoryWithOpts } from "../scripts/hardhat/utils";
import {
  getRequiredEnvVar,
  LogContractDeployment,
  tryVerifyContract,
  tryVerifyContractWithConstructorArgs,
} from "../common/helpers";
import { toBeHex } from "ethers";
import { Mimc } from "contracts/typechain-types";

const func: DeployFunction = async function () {
  const contractName = getRequiredEnvVar("VERIFIER_CONTRACT_NAME");
  const verifierIndex = getRequiredEnvVar("VERIFIER_PROOF_TYPE");
  const provider = ethers.provider;
  const chainId = getRequiredEnvVar("VERIFIER_CHAIN_ID");
  const baseFee = getRequiredEnvVar("VERIFIER_BASE_FEE");
  const coinbase = getRequiredEnvVar("VERIFIER_COINBASE");
  const l2MessageServiceAddress = getRequiredEnvVar("L2_MESSAGE_SERVICE_ADDRESS");

  const mimc = (await deployFromFactory("Mimc", provider)) as Mimc;

  await tryVerifyContract(await mimc.getAddress());

  const constructorArgs = [
    [
      {
        value: toBeHex(chainId, 32),
        name: "chainId",
      },
      {
        value: toBeHex(baseFee, 32),
        name: "baseFee",
      },
      {
        value: toBeHex(coinbase, 32),
        name: "coinbase",
      },
      {
        value: toBeHex(l2MessageServiceAddress, 32),
        name: "l2MessageServiceAddress",
      },
    ],
  ];

  const contract = await deployFromFactoryWithOpts(
    contractName,
    provider,
    {
      libraries: { Mimc: await mimc.getAddress() },
    },
    ...constructorArgs,
  );

  await LogContractDeployment(contractName, contract);
  const contractAddress = await contract.getAddress();

  process.env.PLONKVERIFIER_ADDRESS = contractAddress;

  const setVerifierAddress = ethers.concat([
    "0xc2116974",
    ethers.AbiCoder.defaultAbiCoder().encode(["address", "uint256"], [contractAddress, verifierIndex]),
  ]);

  console.log("setVerifierAddress calldata:", setVerifierAddress);

  await tryVerifyContractWithConstructorArgs(contractAddress, contractName, constructorArgs, {
    Mimc: await mimc.getAddress(),
  });
};
export default func;
func.tags = ["PlonkVerifier"];
