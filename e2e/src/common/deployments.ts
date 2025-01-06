import { AbiCoder, AbstractSigner, BaseContract, ContractFactory, Wallet, ethers } from "ethers";
import { ProxyAdmin__factory, TransparentUpgradeableProxy__factory, ProxyAdmin } from "../typechain";

export const encodeData = (types: string[], values: unknown[], packed?: boolean) => {
  if (packed) {
    return ethers.solidityPacked(types, values);
  }
  return AbiCoder.defaultAbiCoder().encode(types, values);
};

function getInitializerData(contractInterface: ethers.Interface, args: unknown[]) {
  const initializer = "initialize";
  const fragment = contractInterface.getFunction(initializer);
  return contractInterface.encodeFunctionData(fragment!, args);
}

export const encodeLibraryName = (libraryName: string) => {
  const encodedLibraryName = ethers.keccak256(encodeData(["string"], [libraryName])).slice(2, 36);
  return `__$${encodedLibraryName}$__`;
};

export const deployContract = async <T extends ContractFactory>(
  contractFactory: T,
  deployer: AbstractSigner,
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  args?: any[],
): Promise<BaseContract> => {
  const deploymentArgs = args || [];
  const instance = await contractFactory.connect(deployer).deploy(...deploymentArgs);
  await instance.waitForDeployment();
  return instance;
};

const deployUpgradableContract = async <T extends ContractFactory>(
  contractFactory: T,
  deployer: Wallet,
  admin: ProxyAdmin,
  initializerData = "0x",
): Promise<BaseContract> => {
  const instance = await contractFactory.connect(deployer).deploy();
  await instance.waitForDeployment();

  const proxy = await new TransparentUpgradeableProxy__factory()
    .connect(deployer)
    .deploy(await instance.getAddress(), await admin.getAddress(), initializerData);
  await proxy.waitForDeployment();

  return instance.attach(await proxy.getAddress());
};

export async function deployUpgradableContractWithProxyAdmin<T extends ContractFactory>(
  contractFactory: T,
  deployer: Wallet,
  args: unknown[],
) {
  const proxyFactory = new ProxyAdmin__factory(deployer);
  const proxyAdmin = await proxyFactory.connect(deployer).deploy();
  await proxyAdmin.waitForDeployment();
  logger.info(`ProxyAdmin contract deployed. address=${await proxyAdmin.getAddress()}`);

  const contract = await deployUpgradableContract(
    contractFactory,
    deployer,
    proxyAdmin,
    getInitializerData(contractFactory.interface, args),
  );
  logger.info(`Contract deployed. address=${await contract.getAddress()}`);
  return contract;
}
