import { toBeHex } from "ethers";
import { ethers } from "hardhat";
import { HardhatRuntimeEnvironment } from "hardhat/types";
import { DeployFunction } from "hardhat-deploy/types";

import {
  getRequiredEnvVar,
  requireAddressFromRegistryOrEnv,
  setHandoffAddress,
  LogContractDeployment,
  tryVerifyContractWithConstructorArgs,
} from "../common/helpers";
import { getUiSigner, withSignerUiSession } from "../scripts/hardhat/signer-ui-bridge";
import { deployFromFactoryWithOpts } from "../scripts/hardhat/utils";

const func: DeployFunction = withSignerUiSession(
  "01_deploy_PlonkVerifier.ts",
  async function (hre: HardhatRuntimeEnvironment) {
    const signer = await getUiSigner(hre);
    const contractName = getRequiredEnvVar("VERIFIER_CONTRACT_NAME");
    const verifierIndex = getRequiredEnvVar("VERIFIER_PROOF_TYPE");
    const chainId = getRequiredEnvVar("VERIFIER_CHAIN_ID");
    const baseFee = getRequiredEnvVar("VERIFIER_BASE_FEE");
    const coinbase = getRequiredEnvVar("VERIFIER_COINBASE");
    const l2MessageServiceAddress = requireAddressFromRegistryOrEnv(
      hre.network.name,
      "L2MessageService",
      "L2_MESSAGE_SERVICE_ADDRESS",
    );
    const isAllowedCircuitId = getRequiredEnvVar("VERIFIER_IS_ALLOWED_CIRCUIT_ID");

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
        {
          value: toBeHex(isAllowedCircuitId, 32),
          name: "isAllowedCircuitId",
        },
      ],
    ];

    const contract = await deployFromFactoryWithOpts(contractName, signer, {}, ...constructorArgs);

    await LogContractDeployment(contractName, contract);
    const contractAddress = await contract.getAddress();

    setHandoffAddress("VERIFIER_ADDRESS", contractAddress);

    const setVerifierAddress = ethers.concat([
      "0xc2116974",
      ethers.AbiCoder.defaultAbiCoder().encode(["address", "uint256"], [contractAddress, verifierIndex]),
    ]);

    console.log("setVerifierAddress calldata:", setVerifierAddress);

    await tryVerifyContractWithConstructorArgs(contractAddress, contractName, constructorArgs);
  },
);
export default func;
func.tags = ["PlonkVerifier"];
