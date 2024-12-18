import { DeployFunction } from "hardhat-deploy/types";
import { HardhatRuntimeEnvironment } from "hardhat/types";
import { deployUpgradableFromFactory } from "../scripts/hardhat/utils";
import {
  LogContractDeployment,
  getDeployedContractAddress,
  getRequiredEnvVar,
  tryStoreAddress,
  tryVerifyContract,
} from "contracts/common/helpers";

const func: DeployFunction = async function (hre: HardhatRuntimeEnvironment) {
  const { deployments } = hre;

  const contractName = "LineaRollupV5";
  const verifierName = "PlonkVerifier";
  const existingContractAddress = await getDeployedContractAddress(contractName, deployments);
  let verifierAddress = await getDeployedContractAddress(verifierName, deployments);
  if (verifierAddress === undefined) {
    if (process.env["PLONKVERIFIER_ADDRESS"] !== undefined) {
      console.log(`Using environment variable for PlonkVerifier , ${process.env["PLONKVERIFIER_ADDRESS"]}`);
      verifierAddress = process.env["PLONKVERIFIER_ADDRESS"];
    } else {
      throw "Missing PLONKVERIFIER_ADDRESS environment variable";
    }
  } else {
    console.log(`Using deployed variable for PlonkVerifier , ${verifierAddress}`);
  }

  // LineaRollup DEPLOYED AS UPGRADEABLE PROXY
  const LineaRollup_initialStateRootHash = getRequiredEnvVar("LINEA_ROLLUP_INITIAL_STATE_ROOT_HASH");
  const LineaRollup_initialL2BlockNumber = getRequiredEnvVar("LINEA_ROLLUP_INITIAL_L2_BLOCK_NUMBER");
  const LineaRollup_securityCouncil = getRequiredEnvVar("LINEA_ROLLUP_SECURITY_COUNCIL");
  const LineaRollup_operators = getRequiredEnvVar("LINEA_ROLLUP_OPERATORS");
  const LineaRollup_rateLimitPeriodInSeconds = getRequiredEnvVar("LINEA_ROLLUP_RATE_LIMIT_PERIOD");
  const LineaRollup_rateLimitAmountInWei = getRequiredEnvVar("LINEA_ROLLUP_RATE_LIMIT_AMOUNT");
  const LineaRollup_genesisTimestamp = getRequiredEnvVar("LINEA_ROLLUP_GENESIS_TIMESTAMP");

  console.log(`Setting operators ${LineaRollup_operators}`);

  if (existingContractAddress === undefined) {
    console.log(`Deploying initial version, NB: the address will be saved if env SAVE_ADDRESS=true.`);
  } else {
    console.log(`Deploying new version, NB: ${existingContractAddress} will be overwritten if env SAVE_ADDRESS=true.`);
  }
  const contract = await deployUpgradableFromFactory(
    "LineaRollupV5",
    [
      LineaRollup_initialStateRootHash,
      LineaRollup_initialL2BlockNumber,
      verifierAddress,
      LineaRollup_securityCouncil,
      LineaRollup_operators?.split(","),
      LineaRollup_rateLimitPeriodInSeconds,
      LineaRollup_rateLimitAmountInWei,
      LineaRollup_genesisTimestamp,
    ],
    {
      initializer: "initialize(bytes32,uint256,address,address,address[],uint256,uint256,uint256)",
      unsafeAllow: ["constructor"],
    },
  );

  await LogContractDeployment(contractName, contract);
  const contractAddress = await contract.getAddress();

  await tryStoreAddress(hre.network.name, contractName, contractAddress, contract.deploymentTransaction()!.hash);

  await tryVerifyContract(contractAddress);
};

export default func;
func.tags = ["LineaRollupV5"];
