// Get smart contract Errors (with code) from smart contract ABI
// Usage - `pnpm exec tsx scripts/operational/getSmartContractErrorsFromABI.ts <ABI_JSON_FILE_PATH>`

import { readFileSync } from "fs";
import { type Abi, toFunctionSelector } from "viem";
import { formatAbiItem } from "viem/utils";

type SmartContractErrorOutput = {
  name: string;
  functionSignature: string;
  selector: string;
};

function getSmartContractErrorsFromABI(abi: Abi): SmartContractErrorOutput[] {
  return abi.flatMap((item) => {
    if (item.type !== "error") {
      return [];
    }
    const functionSignature = formatAbiItem(item);
    return [{ name: item.name, functionSignature, selector: toFunctionSelector(functionSignature).slice(2) }];
  });
}

function importABIFromCLIArg(): Abi {
  const filePath = process.argv[2];
  if (!filePath) {
    throw new Error(
      "No file path provided. Usage: pnpm exec tsx scripts/operational/getSmartContractErrorsFromABI.ts <ABI_JSON_FILE_PATH>",
    );
  }

  try {
    const artifactJson: { abi: Abi } = JSON.parse(readFileSync(filePath, "utf8"));
    return artifactJson.abi;
  } catch (error: unknown) {
    throw new Error(`Failed to import ABI from ${filePath}: ${error}`);
  }
}

console.log(getSmartContractErrorsFromABI(importABIFromCLIArg()));
