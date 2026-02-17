import { SignerWithAddress } from "@nomicfoundation/hardhat-ethers/signers";
import { DeployProxyOptions } from "@openzeppelin/hardhat-upgrades/dist/utils";
import { ContractFactory, JsonRpcProvider } from "ethers";
import { ethers, upgrades } from "hardhat";
import { FactoryOptions, HardhatEthersHelpers } from "hardhat/types";

async function deployFromFactory(
  contractName: string,
  provider: JsonRpcProvider | HardhatEthersHelpers["provider"] | null = null,
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  ...args: any[]
) {
  const startTime = performance.now();
  const skipLog = process.env.SKIP_DEPLOY_LOG === "true" || false;
  if (!skipLog) {
    const signer = await provider?.getSigner();
    console.log(`Going to deploy ${contractName} with account ${await signer?.getAddress()}...`);
  }

  const factory = await ethers.getContractFactory(contractName);
  if (provider) {
    factory.connect(await provider.getSigner());
  }
  const contract = await factory.deploy(...args);
  if (!skipLog) {
    const deployTx = contract.deploymentTransaction();

    console.log(`${contractName} deployment transaction has been sent, waiting...`, {
      nonce: deployTx?.nonce,
      hash: deployTx?.hash,
      gasPrice: deployTx?.gasPrice?.toString(),
      maxFeePerGas: deployTx?.maxFeePerGas?.toString(),
      maxPriorityFeePerGas: deployTx?.maxPriorityFeePerGas?.toString(),
      gasLimit: deployTx?.gasLimit.toString(),
    });
  }
  const afterDeploy = await contract.waitForDeployment();
  const timeDiff = performance.now() - startTime;
  if (!skipLog) {
    console.log(
      `${contractName} deployed: time=${timeDiff / 1000}s blockNumber=${afterDeploy.deploymentTransaction()?.blockNumber}` +
        ` tx-hash=${afterDeploy.deploymentTransaction()?.hash}`,
    );
  }
  return contract;
}

async function deployFromFactoryWithOpts(
  contractName: string,
  provider: JsonRpcProvider | HardhatEthersHelpers["provider"] | null = null,
  factoryOpts: FactoryOptions,
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  ...args: any[]
) {
  const startTime = performance.now();
  const skipLog = process.env.SKIP_DEPLOY_LOG === "true" || false;
  if (!skipLog) {
    const signer = await provider?.getSigner();
    console.log(`Going to deploy ${contractName} with account ${await signer?.getAddress()}...`);
  }

  const factory = await ethers.getContractFactory(contractName, factoryOpts);
  if (provider) {
    factory.connect(await provider.getSigner());
  }
  const contract = await factory.deploy(...args);
  if (!skipLog) {
    const deployTx = contract.deploymentTransaction();

    console.log(`${contractName} deployment transaction has been sent, waiting...`, {
      nonce: deployTx?.nonce,
      hash: deployTx?.hash,
      gasPrice: deployTx?.gasPrice?.toString(),
      maxFeePerGas: deployTx?.maxFeePerGas?.toString(),
      maxPriorityFeePerGas: deployTx?.maxPriorityFeePerGas?.toString(),
      gasLimit: deployTx?.gasLimit.toString(),
    });
  }
  const afterDeploy = await contract.waitForDeployment();
  const timeDiff = performance.now() - startTime;
  if (!skipLog) {
    console.log(
      `${contractName} deployed: time=${timeDiff / 1000}s blockNumber=${afterDeploy.deploymentTransaction()?.blockNumber}` +
        ` tx-hash=${afterDeploy.deploymentTransaction()?.hash}`,
    );
  }
  return contract;
}

async function deployUpgradableFromFactory(
  contractName: string,
  args?: unknown[],
  opts?: DeployProxyOptions,
  factoryOpts?: FactoryOptions,
) {
  const startTime = performance.now();
  const skipLog = process.env.SKIP_DEPLOY_LOG === "true" || false;
  if (!skipLog) {
    console.log(`Going to deploy upgradable ${contractName}`);
  }
  const factory = await ethers.getContractFactory(contractName, factoryOpts);
  const contract = await upgrades.deployProxy(factory, args, opts);
  if (!skipLog) {
    const deployTx = contract.deploymentTransaction();
    console.log(`Upgradable ${contractName} deployment transaction has been sent, waiting...`, {
      hash: deployTx?.hash,
      gasPrice: deployTx?.gasPrice?.toString(),
      gasLimit: deployTx?.gasLimit.toString(),
    });
  }
  const afterDeploy = await contract.waitForDeployment();
  const timeDiff = performance.now() - startTime;
  if (!skipLog) {
    console.log(
      `${contractName} artifact has been deployed in ${timeDiff / 1000}s` +
        ` tx-hash=${afterDeploy.deploymentTransaction()?.hash}`,
    );
  }
  return contract;
}

async function deployUpgradableWithAbiAndByteCode(
  deployer: SignerWithAddress,
  contractName: string,
  abi: string,
  byteCode: string,
  args?: unknown[],
  opts?: DeployProxyOptions,
) {
  const skipLog = process.env.SKIP_DEPLOY_LOG === "true" || false;
  if (!skipLog) {
    console.log(`Going to deploy upgradable ${contractName}`);
  }
  const factory: ContractFactory = new ContractFactory(abi, byteCode, deployer);

  const contract = await upgrades.deployProxy(factory, args, opts);

  if (!skipLog) {
    const deployTx = contract.deploymentTransaction();
    console.log(`Upgradable ${contractName} deployment transaction has been sent, waiting...`, {
      hash: deployTx?.hash,
      gasPrice: deployTx?.gasPrice?.toString(),
      gasLimit: deployTx?.gasLimit.toString(),
    });
  }
  const afterDeploy = await contract.waitForDeployment();
  if (!skipLog) {
    console.log(`${contractName} artifact has been deployed in tx-hash=${afterDeploy.deploymentTransaction()?.hash}`);
  }
  return contract;
}

async function deployUpgradableFromFactoryWithConstructorArgs(
  contractName: string,
  constructorArgs: unknown[] = [],
  initializerArgs: unknown[] = [],
  opts: DeployProxyOptions = {},
  factoryOpts?: FactoryOptions,
) {
  const startTime = performance.now();
  const skipLog = process.env.SKIP_DEPLOY_LOG === "true" || false;
  if (!skipLog) {
    console.log(`Going to deploy upgradable ${contractName}`);
  }
  const factory = await ethers.getContractFactory(contractName, factoryOpts);
  const contract = await upgrades.deployProxy(factory, initializerArgs, {
    ...opts,
    constructorArgs,
  });
  if (!skipLog) {
    const deployTx = contract.deploymentTransaction();
    console.log(`Upgradable ${contractName} deployment transaction has been sent, waiting...`, {
      hash: deployTx?.hash,
      gasPrice: deployTx?.gasPrice?.toString(),
      gasLimit: deployTx?.gasLimit.toString(),
    });
  }
  const afterDeploy = await contract.waitForDeployment();
  const timeDiff = performance.now() - startTime;
  if (!skipLog) {
    console.log(
      `${contractName} artifact has been deployed in ${timeDiff / 1000}s` +
        ` tx-hash=${afterDeploy.deploymentTransaction()?.hash}`,
    );
  }
  return contract;
}

function requireEnv(name: string): string {
  const envVariable = process.env[name];
  if (!envVariable) {
    throw new Error(`Missing ${name} environment variable`);
  }

  return envVariable;
}

export {
  deployFromFactory,
  deployFromFactoryWithOpts,
  deployUpgradableFromFactory,
  deployUpgradableWithAbiAndByteCode,
  deployUpgradableFromFactoryWithConstructorArgs,
  requireEnv,
};
