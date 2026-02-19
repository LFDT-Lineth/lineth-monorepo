import fs from "fs";
import { ethers } from "hardhat";
import { DeployFunction } from "hardhat-deploy/types";
import { HardhatRuntimeEnvironment } from "hardhat/types";
import path from "path";
import { deployUpgradableWithAbiAndByteCode } from "../scripts/hardhat/utils";
import { tryVerifyContract, getRequiredEnvVar, LogContractDeployment } from "../common/helpers";
import { abi, bytecode } from "./V1/L2MessageServiceV1Deployed.json";

const func: DeployFunction = async function (hre: HardhatRuntimeEnvironment) {
  const mainnetDeployedL2MessageServiceCacheFolder = path.resolve("./deploy/V1/L2MessageServiceV1Cache/");

  const validationFilePath = path.join(hre.config.paths.cache, "validations.json");
  const validationFileBackupPath = path.join(hre.config.paths.cache, "validations_backup.json");

  if (fs.existsSync(validationFilePath)) {
    fs.copyFileSync(validationFilePath, validationFileBackupPath);
  }

  fs.copyFileSync(path.join(mainnetDeployedL2MessageServiceCacheFolder, "validations.json"), validationFilePath);

  const contractName = "L2MessageServiceLineaMainnet";

  const L2MessageService_securityCouncil = getRequiredEnvVar("L2_SECURITY_COUNCIL");
  const L2MessageService_l1l2MessageSetter = getRequiredEnvVar("L2_MESSAGE_SERVICE_L1L2_MESSAGE_SETTER");
  const L2MessageService_rateLimitPeriod = getRequiredEnvVar("L2_MESSAGE_SERVICE_RATE_LIMIT_PERIOD");
  const L2MessageService_rateLimitAmount = getRequiredEnvVar("L2_MESSAGE_SERVICE_RATE_LIMIT_AMOUNT");

  const [deployer] = await ethers.getSigners();

  const contract = await deployUpgradableWithAbiAndByteCode(
    deployer,
    "L2MessageServiceLineaMainnet",
    JSON.stringify(abi),
    bytecode,
    [
      L2MessageService_securityCouncil,
      L2MessageService_l1l2MessageSetter,
      L2MessageService_rateLimitPeriod,
      L2MessageService_rateLimitAmount,
    ],
    {
      initializer: "initialize(address,address,uint256,uint256)",
      unsafeAllow: ["constructor"],
    },
  );

  await LogContractDeployment(contractName, contract);
  const contractAddress = await contract.getAddress();

  await tryVerifyContract(contractAddress);

  fs.unlinkSync(path.join(hre.config.paths.cache, "validations.json"));
  if (fs.existsSync(validationFileBackupPath)) {
    fs.copyFileSync(validationFileBackupPath, validationFilePath);
    fs.unlinkSync(validationFileBackupPath);
  }
};
export default func;
func.tags = ["L2MessageServiceLineaMainnet"];
