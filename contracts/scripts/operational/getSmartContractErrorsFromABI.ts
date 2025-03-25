// Get smart contract Errors (with code) from smart contract ABI
// Usage - `npx ts-node scripts/operational/getSmartContractErrorsFromABI.ts <ABI_JSON_FILE_PATH>`

import { generateFunctionSelector } from "../../common/helpers/hashing";
import { readFileSync } from "fs";

// Input types
type ABIElementInput = {
  internalType: string;
  name: string;
  type: string;
};

type ABIElement = {
  type: string;
  name: string;
  inputs: ABIElementInput[];
};

type ABI = ABIElement[];

type HardhatBuildArtifactJson = {
  abi: ABI;
};

const ERROR_TYPE = "error";

// Output types
type SmartContractErrorOutput = {
  name: string;
  functionSignature: string;
  selector: string;
};

// Function to get smart contract errors from ABI
function getSmartContractErrorsFromABI(abiInput: ABI): SmartContractErrorOutput[] {
  const resp: SmartContractErrorOutput[] = [];

  abiInput.forEach((element: ABIElement) => {
    if (element.type === ERROR_TYPE) {
      const functionSignature = getFunctionSignature(element);
      resp.push({
        name: element.name,
        functionSignature: functionSignature,
        selector: generateFunctionSelector(functionSignature),
      });
    }
  });
  return resp;
}

function getFunctionSignature(abiElement: ABIElement): string {
  let functionSignature = abiElement.name;
  functionSignature += "(";
  abiElement.inputs.forEach((input: ABIElementInput) => {
    functionSignature += input.type;
    functionSignature += ",";
  });
  // Remove trailing "," if have error param/s
  if (abiElement.inputs.length > 0) functionSignature = functionSignature.substring(0, functionSignature.length - 1);
  functionSignature += ")";
  return functionSignature;
}

function importABIFromCLIArg(): ABI {
  const filePath = process.argv[2];
  if (filePath.length === 0)
    throw new Error(
      "No file path provided. Usage: npx ts-node scripts/operational/getSmartContractErrorsFromABI.ts <ABI_JSON_FILE_PATH>",
    );

  try {
    const fileContent = readFileSync(filePath, "utf8");
    const artifactJson: HardhatBuildArtifactJson = JSON.parse(fileContent);
    return artifactJson.abi;
  } catch (error: unknown) {
    throw new Error(`Failed to import ABI from ${filePath}: ${error}`);
  }
}

function main() {
  const abi = importABIFromCLIArg();
  const smartContractErrors = getSmartContractErrorsFromABI(abi);
  console.log(smartContractErrors);
}

main();
