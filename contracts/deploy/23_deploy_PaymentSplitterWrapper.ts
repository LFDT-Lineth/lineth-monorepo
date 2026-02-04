import { ethers } from "hardhat";
import { isAddress } from "ethers";
import { DeployFunction } from "hardhat-deploy/types";
import { getRequiredEnvVar, LogContractDeployment, tryVerifyContractWithConstructorArgs } from "../common/helpers";
import { deployFromFactory } from "../scripts/hardhat/utils";

const func: DeployFunction = async function () {
  const contractName = "PaymentSplitterWrapper";

  const payeesRaw = getRequiredEnvVar("PAYMENT_SPLITTER_PAYEES");
  const sharesRaw = getRequiredEnvVar("PAYMENT_SPLITTER_SHARES");

  // --- Parse comma-separated strings into arrays ---
  const payeesArray = payeesRaw
    .split(",")
    .map((payee) => payee.trim())
    .filter((payee) => payee.length > 0);
  const sharesArray = sharesRaw
    .split(",")
    .map((share) => share.trim())
    .filter((share) => share.length > 0);

  // --- Validate arrays ---
  if (payeesArray.length === 0) {
    throw new Error("Payees array cannot be empty");
  }
  if (sharesArray.length === 0) {
    throw new Error("Shares array cannot be empty");
  }
  if (payeesArray.length !== sharesArray.length) {
    throw new Error(
      `Payees and shares arrays must have the same length. Got ${payeesArray.length} payees and ${sharesArray.length} shares`,
    );
  }

  // Validate addresses
  for (let i = 0; i < payeesArray.length; i++) {
    if (!isAddress(payeesArray[i]!)) {
      throw new Error(`Invalid address at index ${i}: ${payeesArray[i]}`);
    }
  }

  // Validate and parse shares
  const sharesBigInt: bigint[] = [];
  for (let i = 0; i < sharesArray.length; i++) {
    const shareStr = sharesArray[i]!;
    const shareNum = BigInt(shareStr);
    if (shareNum <= 0n) {
      throw new Error(`Share at index ${i} must be positive: ${shareStr}`);
    }
    sharesBigInt.push(shareNum);
  }

  // --- Log params ---
  console.log("Params:");
  console.log("  payees:", payeesArray);
  console.log(
    "  shares:",
    sharesBigInt.map((share) => share.toString()),
  );

  // --- Deploy contract ---
  const contract = await deployFromFactory(contractName, ethers.provider, payeesArray, sharesBigInt);

  await LogContractDeployment(contractName, contract);
  const contractAddress = await contract.getAddress();

  await tryVerifyContractWithConstructorArgs(
    contractAddress,
    "src/operational/PaymentSplitterWrapper.sol:PaymentSplitterWrapper",
    [payeesArray, sharesBigInt],
  );
};

export default func;
func.tags = ["PaymentSplitterWrapper"];
