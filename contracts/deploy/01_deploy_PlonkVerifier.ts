import { toBeHex } from "ethers";
import { ethers } from "hardhat";
import { HardhatRuntimeEnvironment } from "hardhat/types";
import { DeployFunction } from "hardhat-deploy/types";

import {
  getOptionalEnvVar,
  getRequiredEnvVar,
  LogContractDeployment,
  tryVerifyContract,
  tryVerifyContractWithConstructorArgs,
} from "../common/helpers";
import { getUiSigner, withSignerUiSession } from "../scripts/hardhat/signer-ui-bridge";
import { deployFromFactory, deployFromFactoryWithOpts } from "../scripts/hardhat/utils";

const func: DeployFunction = withSignerUiSession(
  "01_deploy_PlonkVerifier.ts",
  async function (hre: HardhatRuntimeEnvironment) {
    const signer = await getUiSigner(hre);
    const contractName = getRequiredEnvVar("VERIFIER_CONTRACT_NAME");
    const verifierIndex = getRequiredEnvVar("VERIFIER_PROOF_TYPE");
    const chainId = getRequiredEnvVar("VERIFIER_CHAIN_ID");
    const baseFee = getRequiredEnvVar("VERIFIER_BASE_FEE");
    const coinbase = getRequiredEnvVar("VERIFIER_COINBASE");
    const l2MessageServiceAddress = getRequiredEnvVar("L2_MESSAGE_SERVICE_ADDRESS");

    const optionalMimcAddress = getOptionalEnvVar("VERIFIER_MIMC_ADDRESS")?.trim();
    let mimcAddress: string;

    if (optionalMimcAddress) {
      if (!ethers.isAddress(optionalMimcAddress)) {
        throw new Error(`VERIFIER_MIMC_ADDRESS must be a valid address, got "${optionalMimcAddress}"`);
      }
      mimcAddress = ethers.getAddress(optionalMimcAddress);
      const code = await ethers.provider.getCode(mimcAddress);
      if (code === "0x") {
        throw new Error(
          `VERIFIER_MIMC_ADDRESS ${mimcAddress} has no contract bytecode on this network; deploy Mimc first or unset VERIFIER_MIMC_ADDRESS to deploy a new library.`,
        );
      }
      console.log(`Reusing existing Mimc library at ${mimcAddress} (VERIFIER_MIMC_ADDRESS)`);
    } else {
      mimcAddress = await (await deployFromFactory("Mimc", signer)).getAddress();
      await tryVerifyContract(mimcAddress);
    }

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
      signer,
      {
        libraries: { Mimc: mimcAddress },
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
      Mimc: mimcAddress,
    });
  },
);
export default func;
func.tags = ["PlonkVerifier"];
