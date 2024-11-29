import { ethers } from "hardhat";
import { LineaRollupInit__factory, LineaRollup__factory, TimeLock__factory } from "../../typechain-types";

/*******************************USAGE******************************************************************
SEPOLIA_PRIVATE_KEY=<your_private_key> \
INFURA_API_KEY=<your_infura_key> \
npx hardhat run scripts/gnosis/encodingTX2.ts --network sepolia

or

LINEA_SEPOLIA_PRIVATE_KEY=<your_private_key> \
INFURA_API_KEY=<your_infura_key> \
npx hardhat run scripts/gnosis/encodingTX2.ts --network linea_sepolia
*******************************************************************************************************/

//--------------------------------------Config------------------------------------

const main = async () => {
  const initialL2BlockNumber = "1987654321";
  const initialStateRootHash = "0x3450000000000000000000000000000000000000000000000000000000000345";

  const proxyAdminContract = "0xd1A02bfB124F5e3970d46111586100E72e7B56bB";
  const proxyContract = "0x2A5CDCfc38856e2590E9Bd32F54Fa348e5De5f48";
  const NewImplementation = "0x7c7FcafBD36316F4c5608Ec1e353E1aBFCBFf83d";

  const shnarfs = [""]; // bytes32[]
  const finalBockNumbers = [""]; // uint256[]

  console.log("Encoded TX Output:");
  console.log("\n");

  //-------------------------updateDelay on timelock--------------------------
  const newDelayInSeconds = 60;

  const updateDelayOnTimelock = TimeLock__factory.createInterface().encodeFunctionData("updateDelay", [
    newDelayInSeconds,
  ]);

  console.log("updateDelayOnTimelock", updateDelayOnTimelock);

  //-------------------------UpgradeAndCall Directly with initializeParentShnarfsAndFinalizedState--------------------------
  const upgradeCallWithInitializeParentShnarfsAndFinalizedState = ethers.concat([
    "0x9623609d",
    ethers.AbiCoder.defaultAbiCoder().encode(
      ["address", "address", "bytes"],
      [
        proxyContract,
        NewImplementation,
        LineaRollup__factory.createInterface().encodeFunctionData("initializeParentShnarfsAndFinalizedState", [
          shnarfs,
          finalBockNumbers,
        ]),
      ],
    ),
  ]);

  console.log("Encoded upgradeAndCall: ", "\n", upgradeCallWithInitializeParentShnarfsAndFinalizedState);
  console.log("\n");

  //---------------------------Upgrade Directly------------------------------

  const upgradeCallUsingSecurityCouncil = ethers.concat([
    "0x99a88ec4",
    ethers.AbiCoder.defaultAbiCoder().encode(["address", "address"], [proxyContract, NewImplementation]),
  ]);

  console.log("Encoded Upgrade call (directly) from Security Council :", "\n", upgradeCallUsingSecurityCouncil);
  console.log("\n");

  //-----------------------Upgrade Directly with Reinitialization----------------------------------

  const upgradeCallWithReinitializationUsingSecurityCouncil = ethers.concat([
    "0x9623609d",
    ethers.AbiCoder.defaultAbiCoder().encode(
      ["address", "address", "bytes"],
      [
        proxyContract,
        NewImplementation,
        LineaRollupInit__factory.createInterface().encodeFunctionData("initializeV2", [
          initialL2BlockNumber,
          initialStateRootHash,
        ]),
      ],
    ),
  ]);

  console.log(
    "Encoded upgradeAndCall (directly) with Reinitialization from Security Council :",
    "\n",
    upgradeCallWithReinitializationUsingSecurityCouncil,
  );
  console.log("\n");

  // ----------------------Additional config for Schedule/Execute-------------------------------

  const timelockDaysDelay = 0;
  const timelockDelay = timelockDaysDelay * 24 * 3600;
  console.log("Schedule/Execute with timelock delay of", timelockDaysDelay);
  console.log("\n");

  //-----------------------------Schedule with initializeParentShnarfsAndFinalizedState----------------------------------
  const scheduleUpgradeInitializeParentShnarfsAndFinalizedStateCallwithZodiac = ethers.concat([
    "0x01d5062a",
    ethers.AbiCoder.defaultAbiCoder().encode(
      ["address", "uint256", "bytes", "bytes32", "bytes32", "uint256"],
      [
        proxyAdminContract,
        0,
        upgradeCallWithInitializeParentShnarfsAndFinalizedState,
        ethers.ZeroHash,
        ethers.ZeroHash,
        timelockDelay,
      ],
    ),
  ]);

  console.log(
    "calling Timelock Schedule with",
    timelockDaysDelay,
    "days delay using Zodiac :",
    "\n",
    scheduleUpgradeInitializeParentShnarfsAndFinalizedStateCallwithZodiac,
  );
  console.log("\n");

  //-------------------------------Execute with Migration Block------------------------------------

  const executeUpgradeInitializeParentShnarfsAndFinalizedStateCallwithZodiac = ethers.concat([
    "0x134008d3",
    ethers.AbiCoder.defaultAbiCoder().encode(
      ["address", "uint256", "bytes", "bytes32", "bytes32"],
      [
        proxyAdminContract,
        0,
        upgradeCallWithInitializeParentShnarfsAndFinalizedState,
        ethers.ZeroHash,
        ethers.ZeroHash,
      ],
    ),
  ]);

  console.log(
    "Encoding to be used for calling Timelock Execute function using Zodiac :",
    "\n",
    executeUpgradeInitializeParentShnarfsAndFinalizedStateCallwithZodiac,
  );
  console.log("\n");

  //----------------------------Schedule--------------------------------------

  const upgradeScheduleCallwithZodiac = ethers.concat([
    "0x01d5062a",
    ethers.AbiCoder.defaultAbiCoder().encode(
      ["address", "uint256", "bytes", "bytes32", "bytes32", "uint256"],
      [proxyAdminContract, 0, upgradeCallUsingSecurityCouncil, ethers.ZeroHash, ethers.ZeroHash, timelockDelay],
    ),
  ]);

  console.log("Delay is set to:", timelockDelay);

  console.log(
    "Encoded schedule Upgrade using Zodiac with ",
    timelockDaysDelay,
    "day delay:",
    "\n",
    upgradeScheduleCallwithZodiac,
  );
  console.log("\n");

  // -------------------------------Execute------------------------------------------

  const upgradeExecuteCallwithZodiac = ethers.concat([
    "0x134008d3",
    ethers.AbiCoder.defaultAbiCoder().encode(
      ["address", "uint256", "bytes", "bytes32", "bytes32"],
      [proxyAdminContract, 0, upgradeCallUsingSecurityCouncil, ethers.ZeroHash, ethers.ZeroHash],
    ),
  ]);

  console.log("Encoded execute Upgrade using Zodiac", "\n", upgradeExecuteCallwithZodiac);
  console.log("\n");
};

main()
  .then(() => process.exit(0))
  .catch((error) => {
    console.error(error);
    process.exit(1);
  });
