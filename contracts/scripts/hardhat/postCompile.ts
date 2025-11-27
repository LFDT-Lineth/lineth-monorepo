import "colors";
import fs from "fs";
import path from "path";
import { artifacts } from "hardhat";
import Diff from "diff";

const UNCHANGED_COLOR = "grey";
const MAX_UNCHANGED_PART_LEN = 100;

const EXPOSED_CONTRACTS = ["L2MessageService", "LineaRollup", "TimeLock", "Validium"];
const CONTRACT_OUPUT_ABIS: { [contractName: string]: string } = {
  L2MessageService: "L2MessageServiceV1.0.abi",
  LineaRollup: "LineaRollupV6.0.abi", // next version
  TimeLock: "TimeLock.abi",
  Validium: "ValidiumV1.0.abi",
};

async function main() {
  const checkOnly = process.env.CHECK_ONLY === "1";

  for (const contract of EXPOSED_CONTRACTS) {
    const abiPath = path.resolve("abi", CONTRACT_OUPUT_ABIS[contract]);

    if (checkOnly) {
      const currentAbi = JSON.parse(fs.readFileSync(abiPath, "utf8"));
      const stringifiedCurrentAbi = JSON.stringify(currentAbi, null, 2);

      const { stringifiedAbi, abi } = await readAbi(contract);

      if (stringifiedAbi !== stringifiedCurrentAbi) {
        showDiff(abi, currentAbi);

        throw new Error(`${contract} ABI has changed, please update it and commit the changes to the repository`);
      }
    } else {
      const { stringifiedAbi } = await readAbi(contract);

      fs.writeFileSync(abiPath, stringifiedAbi);
    }
  }
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
function showDiff(abi: any[], currentAbi: any[]) {
  const diff = Diff.diffJson(abi, currentAbi);

  diff.forEach((part) => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    let color: any;

    if (part.added) {
      color = "green";
    } else if (part.removed) {
      color = "red";
    } else {
      color = UNCHANGED_COLOR;
    }

    let text = part.value;
    if (color === UNCHANGED_COLOR) {
      text = hideDiffChange(text);
    }

    console.error(text[color]);
  });
}

async function readAbi(contract: string) {
  const artifact = await artifacts.readArtifact(contract);
  const abi = artifact.abi;

  const stringifiedAbi = JSON.stringify(abi, null, 2);

  return { stringifiedAbi, abi };
}

function hideDiffChange(diffPart: string) {
  if (diffPart.length <= MAX_UNCHANGED_PART_LEN) {
    return diffPart;
  }

  const firstPart = diffPart.slice(0, MAX_UNCHANGED_PART_LEN / 2);
  const endPart = diffPart.slice(diffPart.length - MAX_UNCHANGED_PART_LEN / 2);

  return `${firstPart}...${endPart}`;
}

main()
  .then(() => process.exit(0))
  .catch((error) => {
    console.error(error);
    process.exit(1);
  });
