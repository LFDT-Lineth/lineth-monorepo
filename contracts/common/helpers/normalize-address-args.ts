import { ContractFactory, getAddress } from "ethers";

/**
 * Checksums ABI address constructor args before ethers attempts name resolution.
 * Prevents ethers v6 from calling HardhatEthersProvider.resolveName (not implemented)
 * when address-typed constructor args are passed as non-checksummed hex strings.
 */
export async function normalizeAddressArgs(factory: ContractFactory, args: unknown[]): Promise<unknown[]> {
  const constructorInputs = factory.interface.deploy.inputs;
  const hasDeployOverrides = args.length === constructorInputs.length + 1;
  const constructorArgs = hasDeployOverrides ? args.slice(0, constructorInputs.length) : args;
  const deployOverrides = hasDeployOverrides ? args.slice(constructorInputs.length) : [];

  if (constructorArgs.length !== constructorInputs.length) {
    return args;
  }

  const normalizedConstructorArgs = await Promise.all(
    constructorInputs.map((input, index) =>
      input.walkAsync(constructorArgs[index], (type, value) => {
        if (type !== "address" || typeof value !== "string") {
          return value;
        }

        return getAddress(value);
      }),
    ),
  );

  return [...normalizedConstructorArgs, ...deployOverrides];
}
