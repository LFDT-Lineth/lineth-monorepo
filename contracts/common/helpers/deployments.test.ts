import { AbstractSigner } from "ethers";
import fs from "fs";
import * as assert from "node:assert/strict";
import os from "os";
import path from "path";

import { getDeployNonceFromEnv, loadArtifactFromDirectory } from "./deployments";

type TestCase = {
  name: string;
  run: () => Promise<void> | void;
};

const ENV_NAME = "LINETH_DEPLOY_NONCE_TEST_NONCE";

function withEnv(value: string | undefined, run: () => Promise<void> | void): Promise<void> | void {
  const previous = process.env[ENV_NAME];
  if (value === undefined) {
    delete process.env[ENV_NAME];
  } else {
    process.env[ENV_NAME] = value;
  }
  const restore = () => {
    if (previous === undefined) {
      delete process.env[ENV_NAME];
    } else {
      process.env[ENV_NAME] = previous;
    }
  };
  try {
    const result = run();
    if (result instanceof Promise) {
      return result.finally(restore);
    }
    restore();
    return result;
  } catch (error) {
    restore();
    throw error;
  }
}

function mockWallet(liveNonce: number): AbstractSigner {
  return {
    getNonce: async () => liveNonce,
  } as unknown as AbstractSigner;
}

const tests: TestCase[] = [
  {
    name: "getDeployNonceFromEnv uses the env value plus offset when set",
    run: async () => {
      await withEnv("10", async () => {
        assert.equal(await getDeployNonceFromEnv(mockWallet(999), ENV_NAME), 10);
        assert.equal(await getDeployNonceFromEnv(mockWallet(999), ENV_NAME, 5), 15);
      });
    },
  },
  {
    name: "getDeployNonceFromEnv falls back to the live wallet nonce when unset or empty (offset ignored)",
    run: async () => {
      await withEnv(undefined, async () => {
        assert.equal(await getDeployNonceFromEnv(mockWallet(42), ENV_NAME), 42);
        assert.equal(await getDeployNonceFromEnv(mockWallet(42), ENV_NAME, 3), 42);
      });
      await withEnv("", async () => {
        assert.equal(await getDeployNonceFromEnv(mockWallet(7), ENV_NAME, 1), 7);
      });
    },
  },
  {
    name: "getDeployNonceFromEnv throws on non-integer env values",
    run: async () => {
      for (const value of ["abc", "5x", "-1", "1e3", "1.5"]) {
        await withEnv(value, async () => {
          await assert.rejects(() => getDeployNonceFromEnv(mockWallet(0), ENV_NAME), /must be a non-negative integer/);
        });
      }
    },
  },
  {
    name: "loadArtifactFromDirectory reads a JSON artifact from disk",
    run: () => {
      const dir = fs.mkdtempSync(path.join(os.tmpdir(), "lineth-artifact-test-"));
      try {
        const artifact = { abi: [{ type: "function", name: "foo" }], bytecode: "0x1234" };
        fs.writeFileSync(path.join(dir, "Foo.json"), JSON.stringify(artifact));
        assert.deepEqual(loadArtifactFromDirectory(dir, "Foo"), artifact);
      } finally {
        fs.rmSync(dir, { recursive: true, force: true });
      }
    },
  },
  {
    name: "loadArtifactFromDirectory throws with folder and contract name when the file is missing",
    run: () => {
      const dir = fs.mkdtempSync(path.join(os.tmpdir(), "lineth-artifact-test-"));
      try {
        assert.throws(
          () => loadArtifactFromDirectory(dir, "Missing"),
          (error: unknown) => {
            const message = error instanceof Error ? error.message : String(error);
            return message.includes("Missing") && message.includes(dir);
          },
        );
      } finally {
        fs.rmSync(dir, { recursive: true, force: true });
      }
    },
  },
];

async function main() {
  for (const test of tests) {
    await test.run();
    console.log(`ok - ${test.name}`);
  }
}

main().catch((error) => {
  console.error(error instanceof Error ? error.stack || error.message : String(error));
  process.exit(1);
});
