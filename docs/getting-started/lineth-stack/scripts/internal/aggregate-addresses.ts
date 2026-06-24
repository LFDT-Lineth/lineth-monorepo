import * as fs from "node:fs";
import * as path from "node:path";

type Lane = "l1" | "l2";

type AddressBook = {
  _meta: {
    l1ChainId: string;
    l2ChainId: string;
    l1RpcUrl: "<redacted>";
    l2RpcUrl: string;
    generatedAt: string;
  };
  l1: Record<string, string>;
  l2: Record<string, string>;
};

const [, , logDir, outPath, l1ChainId, l2ChainId, l2Url] = process.argv;

if (!logDir || !outPath || !l1ChainId || !l2ChainId || !l2Url) {
  throw new Error("usage: aggregate-addresses.ts <logDir> <outPath> <l1ChainId> <l2ChainId> <l2Url>");
}

const deployedLine = /^contract=(\S+)\s+deployed:\s+address=(0x[a-fA-F0-9]{40})\s+blockNumber=(\d+)\s+chainId=(\d+)/;

const result: AddressBook = {
  _meta: {
    l1ChainId,
    l2ChainId,
    l1RpcUrl: "<redacted>",
    l2RpcUrl: l2Url,
    generatedAt: new Date().toISOString(),
  },
  l1: {},
  l2: {},
};

function outputName(contractName: string): string {
  return contractName === "TestERC20" ? "ERC20Example" : contractName;
}

for (const entry of fs.readdirSync(logDir).sort()) {
  if (!entry.endsWith(".log")) continue;

  const lines = fs.readFileSync(path.join(logDir, entry), "utf8").split("\n");
  for (const line of lines) {
    const match = line.match(deployedLine);
    if (!match) continue;

    const [, contractName, address, , chainId] = match;
    const lane: Lane = chainId === l1ChainId ? "l1" : "l2";
    const name = outputName(contractName);

    result[lane][name] = address;
    if (contractName === "TestERC20") {
      result[lane].TestERC20 = address;
    }
  }
}

fs.writeFileSync(outPath, `${JSON.stringify(result, null, 2)}\n`);
console.log("[deploy-contracts] wrote", outPath);
console.log("[deploy-contracts] L1 contracts:", Object.keys(result.l1).join(", ") || "(none)");
console.log("[deploy-contracts] L2 contracts:", Object.keys(result.l2).join(", ") || "(none)");
