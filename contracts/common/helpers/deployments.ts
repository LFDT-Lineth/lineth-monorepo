import { ethers, AbstractSigner, Interface, InterfaceAbi, BaseContract } from "ethers";

export function getInitializerData(contractAbi: InterfaceAbi, initializerFunctionName: string, args: unknown[]) {
  const contractInterface = new Interface(contractAbi);
  const fragment = contractInterface.getFunction(initializerFunctionName);

  if (!fragment) {
    return "0x";
  }

  return contractInterface.encodeFunctionData(fragment, args);
}

export async function deployContractFromArtifacts(
  contractName: string,
  abi: ethers.InterfaceAbi,
  bytecode: ethers.BytesLike,
  wallet: AbstractSigner,
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  ...args: ethers.ContractMethodArgs<any[]>
) {
  const factory = new ethers.ContractFactory(abi, bytecode, wallet);
  const contract = await factory.deploy(...args);

  await LogContractDeployment(contractName, contract);

  return contract;
}

export async function LogContractDeployment(contractName: string, contract: BaseContract) {
  const txReceipt = await contract.deploymentTransaction()?.wait();
  if (!txReceipt) {
    throw "Deployment transaction not found.";
  }

  const contractAddress = await contract.getAddress();
  const chainId = (await contract.deploymentTransaction()!.provider.getNetwork()).chainId;
  console.log(
    `contract=${contractName} deployed: address=${contractAddress} blockNumber=${txReceipt.blockNumber} chainId=${chainId}`,
  );
}
