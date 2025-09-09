import { ethers } from "hardhat";
import { LogContractDeployment } from "contracts/common/helpers";

// Deploy on devnet - PRIVATE_KEY=<...> BLOCKCHAIN_NODE=https://rpc.devnet.linea.build npx hardhat run --network zkevm_dev scripts/testEIP7702/deploy_TestEIP7702Delegation.ts

const func = async function () {
  const contractName = "TestEIP7702Delegation";
  const factory = await ethers.getContractFactory(contractName);
  const contract = await factory.deploy();
  await LogContractDeployment(contractName, contract);
};

func();
