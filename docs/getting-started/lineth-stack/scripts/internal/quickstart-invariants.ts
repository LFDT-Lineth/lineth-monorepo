import { concat, getCreateAddress, isAddress, keccak256 } from "ethers";

const ZERO_32 = `0x${"00".repeat(32)}`;

export type BootPrecomputedAddressInput = {
  l1DeployerAddress: string;
  l1DeployerStartNonce: bigint | number | string;
  l2DeployerAddress: string;
};

export type BootPrecomputedAddresses = {
  l1LineaRollup: string;
  l2MessageService: string;
};

function die(message: string): never {
  console.error(`[quickstart-invariants] ERROR: ${message}`);
  process.exit(1);
}

function assertBytes32(value: string, label: string) {
  if (!/^0x[0-9a-fA-F]{64}$/.test(value)) {
    throw new Error(`${label} must be a 32-byte hex value`);
  }
}

function toNonce(value: bigint | number | string, label: string): bigint {
  try {
    const nonce = BigInt(value);
    if (nonce < 0n) {
      throw new Error("negative nonce");
    }
    return nonce;
  } catch {
    throw new Error(`${label} must be a non-negative integer`);
  }
}

export function computeGenesisShnarf(initialStateRootHash: string): string {
  assertBytes32(initialStateRootHash, "genesis state root");
  return keccak256(concat([ZERO_32, ZERO_32, initialStateRootHash, ZERO_32, ZERO_32]));
}

export function computeBootPrecomputedAddresses(input: BootPrecomputedAddressInput): BootPrecomputedAddresses {
  if (!isAddress(input.l1DeployerAddress)) {
    throw new Error(`l1 deployer address is invalid: ${input.l1DeployerAddress}`);
  }
  if (!isAddress(input.l2DeployerAddress)) {
    throw new Error(`l2 deployer address is invalid: ${input.l2DeployerAddress}`);
  }

  const l1StartNonce = toNonce(input.l1DeployerStartNonce, "l1 deployer start nonce");
  return {
    l1LineaRollup: getCreateAddress({ from: input.l1DeployerAddress, nonce: l1StartNonce + 4n }),
    l2MessageService: getCreateAddress({ from: input.l2DeployerAddress, nonce: 2 }),
  };
}

function main() {
  const [command, value] = process.argv.slice(2);
  if (command === "genesis-shnarf") {
    if (!value) {
      die("usage: quickstart-invariants.ts genesis-shnarf <0xStateRoot>");
    }
    try {
      console.log(computeGenesisShnarf(value));
    } catch (error) {
      die(error instanceof Error ? error.message : String(error));
    }
    return;
  }

  if (command) {
    die(`unknown command: ${command}`);
  }
}

if (require.main === module) {
  main();
}
