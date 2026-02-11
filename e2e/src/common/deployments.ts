import { Client, DeployContractParameters } from "viem";
import { deployContract as viemDeployContract, waitForTransactionReceipt } from "viem/actions";

export function linkBytecode(
  bytecode: `0x${string}`,
  linkReferences: Record<string, Record<string, { start: number; length: number }[]>>,
  libraries: Record<string, `0x${string}`>,
): `0x${string}` {
  let linked = bytecode;

  for (const file of Object.keys(linkReferences)) {
    for (const libName of Object.keys(linkReferences[file])) {
      const address = libraries[libName];
      if (!address) {
        throw new Error(`Missing address for library: ${libName}`);
      }

      const refs = linkReferences[file][libName];

      for (const { start, length } of refs) {
        const addressBytes = address.replace("0x", "").toLowerCase();

        const startPos = 2 + start * 2;
        const endPos = startPos + length * 2;

        linked = linked.slice(0, startPos) + addressBytes + linked.slice(endPos);
      }
    }
  }

  return linked as `0x${string}`;
}

export async function deployContract(client: Client, params: DeployContractParameters) {
  const hash = await viemDeployContract(client, params);
  const receipt = await waitForTransactionReceipt(client, { hash });
  return receipt.contractAddress!;
}
