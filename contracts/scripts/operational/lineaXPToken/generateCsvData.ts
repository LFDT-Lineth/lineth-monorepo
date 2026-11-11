import * as fs from "fs";
import { generatePrivateKey, privateKeyToAddress } from "viem/accounts";

async function main() {
  let data = "recipient,amount\n";
  const values = [10, 15, 7, 9, 20];

  fs.writeFileSync("./scripts/operational/csvFilesToTest/example.csv", data);
  data = "";
  for (let i = 0; i < 2; i++) {
    let value = 23;
    const walletAddress = privateKeyToAddress(generatePrivateKey());
    if (i % 3 == 0) {
      value = values[0];
    }
    if (i % 5 == 0) {
      value = values[1];
    }
    if (i % 7 == 0) {
      value = values[2];
    }
    if (i % 13 == 0) {
      value = values[3];
    }
    if (i % 17 == 0) {
      value = values[4];
    }
    data += `${walletAddress},${value}\n`;
  }
  fs.appendFileSync("example2.csv", data);
}

main()
  .then(() => process.exit(0))
  .catch((error) => {
    console.error(error);
    process.exit(1);
  });
