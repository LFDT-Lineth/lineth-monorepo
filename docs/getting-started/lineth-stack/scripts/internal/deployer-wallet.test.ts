import * as assert from "node:assert/strict";
import { createHash } from "node:crypto";
import * as fs from "node:fs";
import * as os from "node:os";
import * as path from "node:path";

import { encryptKeystoreJson, isAddress, Wallet } from "ethers";

import {
  LOCAL_L1_CONTAINER_RPC_URL,
  LOCAL_L1_DEPLOYER_PRIVATE_KEY,
  LOCAL_L1_HOST_RPC_URL,
  emitShellEnv,
  resolveL1DeployerConfig,
} from "./deployer-wallet";

type TestCase = {
  name: string;
  run: () => Promise<void> | void;
};

const TEST_RPC_URL = "https://example.invalid/rpc";
const LEGACY_PRIVATE_KEY = testPrivateKey("legacy-deployer");
const SECRET_PASSWORD = "secret-password-for-test";

function testPrivateKey(label: string): string {
  return `0x${createHash("sha256").update(`lineth-stack-test:${label}`).digest("hex")}`;
}

function tmpStackDir(): string {
  return fs.mkdtempSync(path.join(os.tmpdir(), "linea-deployer-wallet-"));
}

function read(file: string): string {
  return fs.readFileSync(file, "utf8");
}

async function writeEncryptedKeystore(file: string, privateKey: string, password: string) {
  const wallet = new Wallet(privateKey);
  fs.mkdirSync(path.dirname(file), { recursive: true });
  const encrypted = await encryptKeystoreJson({ address: wallet.address, privateKey: wallet.privateKey }, password, {
    scrypt: { N: 64, r: 8, p: 1 },
  });
  fs.writeFileSync(file, `${encrypted}\n`, { mode: 0o600 });
}

async function expectRejectsWithoutSecrets(run: () => Promise<unknown>, expected: RegExp, secrets: string[]) {
  await assert.rejects(run, (error) => {
    assert.ok(error instanceof Error);
    assert.match(error.message, expected);
    for (const secret of secrets) {
      assert.doesNotMatch(error.message, new RegExp(secret.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")));
    }
    return true;
  });
}

const tests: TestCase[] = [
  {
    name: "generates default Sepolia deployer keystore when missing",
    run: async () => {
      const stackDir = tmpStackDir();
      const accountsDir = path.join(stackDir, "artifacts", "accounts");
      const resolved = await resolveL1DeployerConfig(
        {
          L1_MODE: "sepolia",
          L1_RPC_URL: TEST_RPC_URL,
          LINETH_DEPLOYER_KEYSTORE_SCRYPT_N: "64",
        },
        "host",
        { stackDir, accountsDir },
      );

      assert.equal(resolved.mode, "sepolia");
      assert.equal(resolved.rpcUrl, TEST_RPC_URL);
      assert.equal(resolved.source, "generated-keystore");
      assert.equal(resolved.created, true);
      assert.ok(isAddress(resolved.address));
      assert.match(resolved.privateKey, /^0x[0-9a-f]{64}$/);
      assert.equal(resolved.keystorePath, path.join(accountsDir, "deployer-keystore", "l1-deployer.json"));
      assert.equal(resolved.passwordFilePath, path.join(accountsDir, "deployer-keystore", "password.txt"));
      assert.ok(fs.existsSync(resolved.keystorePath));
      assert.ok(fs.existsSync(resolved.passwordFilePath));
      assert.doesNotMatch(read(resolved.keystorePath), new RegExp(resolved.privateKey.slice(2), "i"));
    },
  },
  {
    name: "reuses existing generated Sepolia deployer keystore",
    run: async () => {
      const stackDir = tmpStackDir();
      const accountsDir = path.join(stackDir, "artifacts", "accounts");
      const env = {
        L1_MODE: "sepolia",
        L1_RPC_URL: TEST_RPC_URL,
        LINETH_DEPLOYER_KEYSTORE_SCRYPT_N: "64",
      };

      const first = await resolveL1DeployerConfig(env, "host", { stackDir, accountsDir });
      const second = await resolveL1DeployerConfig(env, "host", { stackDir, accountsDir });

      assert.equal(second.source, "generated-keystore");
      assert.equal(second.created, false);
      assert.equal(second.address, first.address);
      assert.equal(second.privateKey, first.privateKey);
    },
  },
  {
    name: "honors advanced encrypted deployer keystore override",
    run: async () => {
      const stackDir = tmpStackDir();
      const accountsDir = path.join(stackDir, "artifacts", "accounts");
      const overridePrivateKey = testPrivateKey("override-deployer");
      const keystorePath = path.join(stackDir, "custom", "l1-deployer.json");
      await writeEncryptedKeystore(keystorePath, overridePrivateKey, SECRET_PASSWORD);

      const resolved = await resolveL1DeployerConfig(
        {
          L1_MODE: "sepolia",
          L1_RPC_URL: TEST_RPC_URL,
          L1_DEPLOYER_KEYSTORE_PATH: "custom/l1-deployer.json",
          L1_DEPLOYER_KEYSTORE_PASSWORD: SECRET_PASSWORD,
          LINETH_DEPLOYER_KEYSTORE_SCRYPT_N: "64",
        },
        "host",
        { stackDir, accountsDir },
      );

      assert.equal(resolved.source, "provided-keystore");
      assert.equal(resolved.created, false);
      assert.equal(resolved.keystorePath, keystorePath);
      assert.equal(resolved.privateKey, overridePrivateKey);
      assert.equal(resolved.address, new Wallet(overridePrivateKey).address);
    },
  },
  {
    name: "local mode ignores Sepolia deployer artifact and stale config",
    run: async () => {
      const stackDir = tmpStackDir();
      const accountsDir = path.join(stackDir, "artifacts", "accounts");

      const host = await resolveL1DeployerConfig(
        {
          L1_MODE: "local",
          L1_RPC_URL: TEST_RPC_URL,
          L1_DEPLOYER_PRIVATE_KEY: LEGACY_PRIVATE_KEY,
          L1_DEPLOYER_KEYSTORE_PATH: "missing.json",
          L1_DEPLOYER_KEYSTORE_PASSWORD: "wrong",
        },
        "host",
        { stackDir, accountsDir },
      );
      const container = await resolveL1DeployerConfig({ L1_MODE: "local" }, "container", { stackDir, accountsDir });

      assert.equal(host.source, "local-genesis");
      assert.equal(host.rpcUrl, LOCAL_L1_HOST_RPC_URL);
      assert.equal(host.privateKey, LOCAL_L1_DEPLOYER_PRIVATE_KEY);
      assert.equal(container.rpcUrl, LOCAL_L1_CONTAINER_RPC_URL);
      assert.equal(container.privateKey, LOCAL_L1_DEPLOYER_PRIVATE_KEY);
      assert.equal(fs.existsSync(path.join(accountsDir, "deployer-keystore")), false);
    },
  },
  {
    name: "local mode host RPC honors HOST_PORT_L1_RPC override",
    run: async () => {
      const stackDir = tmpStackDir();
      const accountsDir = path.join(stackDir, "artifacts", "accounts");

      const host = await resolveL1DeployerConfig(
        {
          L1_MODE: "local",
          HOST_PORT_L1_RPC: "9445",
        },
        "host",
        { stackDir, accountsDir },
      );
      const container = await resolveL1DeployerConfig(
        {
          L1_MODE: "local",
          HOST_PORT_L1_RPC: "9445",
        },
        "container",
        { stackDir, accountsDir },
      );

      assert.equal(host.rpcUrl, "http://localhost:9445");
      assert.equal(container.rpcUrl, LOCAL_L1_CONTAINER_RPC_URL);
    },
  },
  {
    name: "malformed keystore and password failures are clear and sanitized",
    run: async () => {
      const stackDir = tmpStackDir();
      const accountsDir = path.join(stackDir, "artifacts", "accounts");
      const badKeystore = path.join(stackDir, "custom", "bad.json");
      fs.mkdirSync(path.dirname(badKeystore), { recursive: true });
      fs.writeFileSync(badKeystore, `not-json-${SECRET_PASSWORD}-${LEGACY_PRIVATE_KEY}\n`);

      await expectRejectsWithoutSecrets(
        () =>
          resolveL1DeployerConfig(
            {
              L1_MODE: "sepolia",
              L1_RPC_URL: TEST_RPC_URL,
              L1_DEPLOYER_KEYSTORE_PATH: "custom/bad.json",
              L1_DEPLOYER_KEYSTORE_PASSWORD: SECRET_PASSWORD,
            },
            "host",
            { stackDir, accountsDir },
          ),
        /Could not decrypt L1 deployer keystore/,
        [SECRET_PASSWORD, LEGACY_PRIVATE_KEY],
      );
    },
  },
  {
    name: "deprecated raw private key compatibility still works when explicitly supplied",
    run: async () => {
      const stackDir = tmpStackDir();
      const accountsDir = path.join(stackDir, "artifacts", "accounts");
      const resolved = await resolveL1DeployerConfig(
        {
          L1_MODE: "sepolia",
          L1_RPC_URL: TEST_RPC_URL,
          L1_DEPLOYER_PRIVATE_KEY: LEGACY_PRIVATE_KEY,
        },
        "host",
        { stackDir, accountsDir },
      );

      assert.equal(resolved.source, "legacy-raw-key");
      assert.equal(resolved.created, false);
      assert.equal(resolved.privateKey, LEGACY_PRIVATE_KEY);
      assert.equal(resolved.address, new Wallet(LEGACY_PRIVATE_KEY).address);
      assert.equal(fs.existsSync(path.join(accountsDir, "deployer-keystore")), false);
    },
  },
  {
    name: "shell env output is quoted and contains no extra logs",
    run: async () => {
      const stackDir = tmpStackDir();
      const accountsDir = path.join(stackDir, "artifacts", "accounts");
      const resolved = await resolveL1DeployerConfig(
        {
          L1_MODE: "sepolia",
          L1_RPC_URL: TEST_RPC_URL,
          L1_DEPLOYER_PRIVATE_KEY: LEGACY_PRIVATE_KEY,
        },
        "host",
        { stackDir, accountsDir },
      );
      const shellEnv = emitShellEnv(resolved);

      assert.match(shellEnv, /^L1_MODE='sepolia'\n/);
      assert.match(shellEnv, /\nL1_DEPLOYER_SOURCE='legacy-raw-key'\n/);
      assert.match(shellEnv, new RegExp(`\nL1_DEPLOYER_PRIVATE_KEY='${LEGACY_PRIVATE_KEY}'\n`));
      assert.doesNotMatch(shellEnv, /secret-password/);
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
  console.error(error instanceof Error ? error.stack ?? error.message : error);
  process.exit(1);
});
