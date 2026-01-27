import { ethers, upgrades } from "hardhat";
import { DeployFunction } from "hardhat-deploy/types";
import { tryVerifyContract, getRequiredEnvVar } from "../common/helpers";
import { L2MessageService__factory } from "contracts/typechain-types";

const func: DeployFunction = async function () {
  const contractName = "L2MessageService";

  const proxyAddress = getRequiredEnvVar("L2MESSAGESERVICE_ADDRESS");

  const factory = await ethers.getContractFactory(contractName);

  console.log("Deploying Contract...");
  const newContract = await upgrades.deployImplementation(factory, {
    kind: "transparent",
  });

  const contract = newContract.toString();

  console.log(`Contract deployed at ${contract}`);

  // The encoding should be used through the safe.
  // THIS IS JUST A SAMPLE AND WILL BE ADJUSTED WHEN NEEDED FOR GENERATING THE CALLDATA FOR THE UPGRADE CALL
  // https://www.4byte.directory/signatures/?bytes4_signature=0x9623609d
  const upgradeCallWithReinitializationUsingSecurityCouncil = ethers.concat([
    "0x9623609d",
    ethers.AbiCoder.defaultAbiCoder().encode(
      ["address", "address", "bytes"],
      [proxyAddress, newContract, L2MessageService__factory.createInterface().encodeFunctionData("reinitializeV3")],
    ),
  ]);

  console.log(
    "Encoded Tx Upgrade with Reinitialization from Security Council:",
    "\n",
    upgradeCallWithReinitializationUsingSecurityCouncil,
  );
  console.log("\n");

  await tryVerifyContract(contract);
};

export default func;
func.tags = ["L2MessageServiceWithReinitialization"];
