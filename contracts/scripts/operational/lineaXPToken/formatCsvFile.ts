import csv from "csv-parser";
import fs from "fs";
import path from "path";
import { isAddress, zeroAddress } from "viem";
import { hideBin } from "yargs/helpers";
import yargs from "yargs/yargs";

const argv = yargs(hideBin(process.argv))
  .option("path", {
    alias: "p",
    describe: "The path to csv file",
    type: "string",
    demandOption: true,
    coerce: (arg) => {
      if (!fs.existsSync(arg)) {
        throw new Error(`File ${arg} does not exist.`);
      }

      if (path.extname(arg) !== ".csv") {
        throw new Error(`File ${arg} is not a CSV file.`);
      }

      const fileContent = fs.readFileSync(arg, "utf8");
      const headers = fileContent.split("\n")[0].split(",");

      const expectedHeaders = ["recipient", "amount"];
      if (!expectedHeaders.every((header) => headers.includes(header))) {
        throw new Error(`CSV file ${arg} does not have the expected headers`);
      }
      return arg;
    },
  })
  .option("output-file", {
    alias: "o",
    describe: "The json file name",
    type: "string",
    demandOption: true,
    coerce: (arg) => {
      if (path.extname(arg) !== ".json") {
        throw new Error(`File ${arg} is not a JSON file.`);
      }

      if (fs.existsSync(arg)) {
        throw new Error(`File ${arg} already exists.`);
      }
      return arg;
    },
  })
  .parseSync();

const BATCH_SIZE = 700;

type Element = {
  batches: string[][];
  numberOfBatches: number;
};

type Batch = {
  id: number;
  recipients: string[];
  amount: number;
};

function isValidAmount(amount: number, currentCsvAmount: number): boolean {
  const intAmount = Number(amount);

  if (!Number.isInteger(intAmount)) {
    return false;
  }

  if (intAmount == 0) {
    return false;
  }

  if (intAmount < currentCsvAmount) {
    return false;
  }

  return true;
}

function checkIfDuplicateExists(recipientArrayValidation: string[]) {
  const printDuplicates = (arr: string[]) => arr.filter((item, index) => arr.indexOf(item) !== index);
  const duplicates = printDuplicates(recipientArrayValidation);

  if (duplicates.length > 0) {
    console.log(duplicates);
  }

  return new Set(recipientArrayValidation).size !== recipientArrayValidation.length;
}

function checkIfValidAddresses(recipient: string): boolean {
  return isAddress(recipient) && recipient !== zeroAddress;
}

function arraysContainSameValues(array1: string[], array2: string[]): boolean {
  const sortedArray1 = array1.slice().sort();
  const sortedArray2 = array2.slice().sort();

  if (sortedArray1.length !== sortedArray2.length) {
    return false;
  }

  return sortedArray1.every((value, index) => value === sortedArray2[index]);
}

async function readAndFormatCSVFile(filePath: string): Promise<{ data: Batch[]; recipientAddressesInCsv: string[] }> {
  const results: { [key: number]: Element } = {};
  let currentCsvTokenAmount = 0;
  const recipientAddressesInCsv: string[] = [];

  return new Promise((resolve, reject) => {
    fs.createReadStream(filePath, "utf-8")
      .pipe(csv())
      .on("data", ({ amount, recipient }) => {
        recipientAddressesInCsv.push(recipient);

        if (!isValidAmount(amount, currentCsvTokenAmount)) {
          throw `Amount ${amount} must be an integer and must be higher than 0.`;
        }

        if (!checkIfValidAddresses(recipient)) {
          throw `Invalid recipient address ${recipient} found in the CSV file`;
        }

        currentCsvTokenAmount = amount;

        const element = results[amount];
        if (element) {
          if (element.batches[element.numberOfBatches - 1].length === BATCH_SIZE) {
            element.batches[element.numberOfBatches] = [recipient];
            element.numberOfBatches++;
            return;
          }
          element.batches[element.numberOfBatches - 1].push(recipient);
          return;
        }

        results[amount] = {
          numberOfBatches: 1,
          batches: [[recipient]],
        };
      })
      .on("end", () => {
        const resultObjectToArray = Object.entries(results).map(([amount, element]) => {
          return {
            ...element,
            amount: parseInt(amount),
          };
        });

        // Split batches into different object with a unique batch id
        let id = 1;
        const data: Batch[] = [];

        for (const item of resultObjectToArray) {
          for (const batch of item.batches) {
            data.push({
              id: id++,
              recipients: batch,
              amount: item.amount,
            });
          }
        }
        return resolve({ data, recipientAddressesInCsv });
      })
      .on("error", (error) => reject(error));
  });
}

async function main(args: typeof argv) {
  const { data, recipientAddressesInCsv } = await readAndFormatCSVFile(args.path);

  const recipientAddresses = Object.values(data).flatMap((value) => value.recipients);

  if (checkIfDuplicateExists(recipientAddresses) || checkIfDuplicateExists(recipientAddressesInCsv)) {
    throw "Duplicate recipients found in the CSV file";
  }

  console.log("No duplicates found in data.");

  if (!arraysContainSameValues(recipientAddresses, recipientAddressesInCsv)) {
    throw "Recipients address are missing in formatted data.";
  }

  console.log("After formatting, the data between original file and formatted file is consistent.");

  fs.writeFileSync(args.outputFile, JSON.stringify(data, null, 2));

  console.log(`A new JSON file has been created: ${args.outputFile}`);
}

main(argv)
  .then(() => process.exit(0))
  .catch((error) => {
    console.error(error);
    process.exit(1);
  });
