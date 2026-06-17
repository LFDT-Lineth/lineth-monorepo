import * as assert from "node:assert/strict";
import * as fs from "node:fs";
import * as path from "node:path";

import {
  LOCAL_L1_CHAIN_ID,
  LOCAL_L1_CONTAINER_RPC_URL,
  LOCAL_L1_DEPLOYER_PRIVATE_KEY,
  LOCAL_L1_HOST_RPC_URL,
  SEPOLIA_POLICY_DEFAULTS,
  buildSepoliaPolicyConfig,
  parseBoolean,
  parseDecimalWei,
  readDotEnvContents,
  resolveL1Config,
  runL1PolicyCheck,
  runSepoliaPolicyCheck,
} from "./sepolia-policy";

type FakeProviderOptions = {
  chainId?: bigint;
  latestNonce?: number;
  pendingNonce?: number;
  balance?: bigint;
  maxFeePerGas?: bigint | null;
  gasPrice?: bigint | null;
  blockNumber?: number;
  blobBaseFee?: bigint;
  blobBaseFeeError?: Error;
};

class FakeProvider {
  private readonly options: Required<Omit<FakeProviderOptions, "blobBaseFee" | "blobBaseFeeError">> &
    Pick<FakeProviderOptions, "blobBaseFee" | "blobBaseFeeError">;

  constructor(options: FakeProviderOptions = {}) {
    this.options = {
      chainId: 11155111n,
      latestNonce: 3,
      pendingNonce: 3,
      balance: 3_000_000_000_000_000_000n,
      maxFeePerGas: 4_000_000_000n,
      gasPrice: 3_000_000_000n,
      blockNumber: 109,
      ...options,
    };
  }

  async getNetwork() {
    return { chainId: this.options.chainId };
  }

  async getTransactionCount(_address: string, blockTag: "latest" | "pending") {
    return blockTag === "pending" ? this.options.pendingNonce : this.options.latestNonce;
  }

  async getBalance() {
    return this.options.balance;
  }

  async getFeeData() {
    return {
      maxFeePerGas: this.options.maxFeePerGas,
      gasPrice: this.options.gasPrice,
    };
  }

  async getBlockNumber() {
    return this.options.blockNumber;
  }

  async send(method: string) {
    assert.equal(method, "eth_blobBaseFee");
    if (this.options.blobBaseFeeError) {
      throw this.options.blobBaseFeeError;
    }
    return this.options.blobBaseFee === undefined ? undefined : `0x${this.options.blobBaseFee.toString(16)}`;
  }
}

type TestCase = {
  name: string;
  run: () => Promise<void> | void;
};

const baseEnv = {
  L1_RPC_URL: "https://example.invalid/rpc",
  L1_DEPLOYER_PRIVATE_KEY: LOCAL_L1_DEPLOYER_PRIVATE_KEY,
};

async function expectRejectsMessage(run: () => Promise<unknown>, expected: string) {
  await assert.rejects(run, (error) => {
    assert.ok(error instanceof Error);
    assert.match(error.message, new RegExp(expected));
    return true;
  });
}

const tests: TestCase[] = [
  {
    name: "L1 mode defaults to Sepolia and requires explicit RPC",
    run: async () => {
      assert.equal((await resolveL1Config(baseEnv, "host")).mode, "sepolia");
      await assert.rejects(
        () => resolveL1Config({ L1_MODE: "bogus" }, "host"),
        /L1_MODE must be one of sepolia, local/,
      );
      await assert.rejects(() => resolveL1Config({ L1_MODE: "sepolia" }, "host"), /L1_RPC_URL must be set/);
    },
  },
  {
    name: "local L1 mode resolves host and container defaults",
    run: async () => {
      const host = await resolveL1Config({ L1_MODE: "local" }, "host");
      const container = await resolveL1Config({ L1_MODE: "local" }, "container");

      assert.equal(host.mode, "local");
      assert.equal(host.rpcUrl, LOCAL_L1_HOST_RPC_URL);
      assert.equal(host.deployerPrivateKey, LOCAL_L1_DEPLOYER_PRIVATE_KEY);
      assert.equal(container.rpcUrl, LOCAL_L1_CONTAINER_RPC_URL);
      assert.equal(container.deployerPrivateKey, LOCAL_L1_DEPLOYER_PRIVATE_KEY);
    },
  },
  {
    name: "local L1 mode ignores stale Sepolia RPC URL",
    run: async () => {
      const staleEnv = {
        L1_MODE: "local",
        L1_RPC_URL: "https://sepolia.example.invalid/rpc",
      };

      assert.equal((await resolveL1Config(staleEnv, "host")).rpcUrl, LOCAL_L1_HOST_RPC_URL);
      assert.equal((await resolveL1Config(staleEnv, "container")).rpcUrl, LOCAL_L1_CONTAINER_RPC_URL);
    },
  },
  {
    name: "local L1 mode honors HOST_PORT_L1_RPC for host checks only",
    run: async () => {
      const env = {
        L1_MODE: "local",
        HOST_PORT_L1_RPC: "9445",
      };

      assert.equal((await resolveL1Config(env, "host")).rpcUrl, "http://localhost:9445");
      assert.equal((await resolveL1Config(env, "container")).rpcUrl, LOCAL_L1_CONTAINER_RPC_URL);
    },
  },
  {
    name: "local L1 mode ignores stale Sepolia deployer private key",
    run: async () => {
      const staleEnv = {
        L1_MODE: "local",
        L1_DEPLOYER_PRIVATE_KEY: "0xabc",
      };

      assert.equal((await resolveL1Config(staleEnv, "host")).deployerPrivateKey, LOCAL_L1_DEPLOYER_PRIVATE_KEY);
      assert.equal((await resolveL1Config(staleEnv, "container")).deployerPrivateKey, LOCAL_L1_DEPLOYER_PRIVATE_KEY);
    },
  },
  {
    name: "local L1 policy accepts local chain and skips Sepolia gas and pending nonce gates",
    run: async () => {
      const report = await runL1PolicyCheck({
        provider: new FakeProvider({
          chainId: LOCAL_L1_CHAIN_ID,
          latestNonce: 4,
          pendingNonce: 5,
          balance: 1n,
          maxFeePerGas: 1_000_000_000_000_000n,
          blobBaseFee: 1_000_000_000_000_000n,
        }),
        deployerAddress: "0x1111111111111111111111111111111111111111",
        env: { L1_MODE: "local" },
      });

      assert.equal(report.mode, "local");
      assert.equal(report.chainId, LOCAL_L1_CHAIN_ID);
      assert.equal(report.latestNonce, 4);
      assert.equal(report.pendingNonce, 5);
      assert.equal(report.minimumBalanceWei, 1n);
      assert.equal(report.currentExecutionFeeWei, undefined);
      assert.equal(report.blobBaseFeeWei, undefined);
    },
  },
  {
    name: "local L1 policy rejects wrong chain and zero deployer balance without Sepolia wording",
    run: async () => {
      await expectRejectsMessage(
        () =>
          runL1PolicyCheck({
            provider: new FakeProvider({ chainId: 11155111n }),
            deployerAddress: "0x1111111111111111111111111111111111111111",
            env: { L1_MODE: "local" },
          }),
        "L1_RPC_URL must point to local L1 chainId 31648428; got 11155111",
      );

      await assert.rejects(
        () =>
          runL1PolicyCheck({
            provider: new FakeProvider({ chainId: LOCAL_L1_CHAIN_ID, balance: 0n }),
            deployerAddress: "0x1111111111111111111111111111111111111111",
            env: { L1_MODE: "local" },
          }),
        (error) => {
          assert.ok(error instanceof Error);
          assert.match(error.message, /has zero wei/);
          assert.doesNotMatch(error.message, /Sepolia/);
          return true;
        },
      );
    },
  },
  {
    name: "valid Sepolia config returns a policy report",
    run: async () => {
      const report = await runSepoliaPolicyCheck({
        provider: new FakeProvider({ blobBaseFee: 2_000_000_000n }),
        deployerAddress: "0x1111111111111111111111111111111111111111",
        env: baseEnv,
      });

      assert.equal(report.chainId, 11155111n);
      assert.equal(report.latestNonce, 3);
      assert.equal(report.pendingNonce, 3);
      assert.equal(report.l1AccountSetupBlockNumber, 109);
      assert.equal(report.l1PostmanListenerStartBlock, 104);
      assert.equal(report.currentExecutionFeeWei, 4_000_000_000n);
      assert.equal(report.blobBaseFeeWei, 2_000_000_000n);
      assert.deepEqual(report.warnings, []);
    },
  },
  {
    name: "non-Sepolia chain ID fails",
    run: async () => {
      await expectRejectsMessage(
        () =>
          runSepoliaPolicyCheck({
            provider: new FakeProvider({ chainId: 1n }),
            deployerAddress: "0x1111111111111111111111111111111111111111",
            env: baseEnv,
          }),
        "L1_RPC_URL must point to Sepolia chainId 11155111; got 1",
      );
    },
  },
  {
    name: "pending nonce fails",
    run: async () => {
      await expectRejectsMessage(
        () =>
          runSepoliaPolicyCheck({
            provider: new FakeProvider({ latestNonce: 4, pendingNonce: 5 }),
            deployerAddress: "0x1111111111111111111111111111111111111111",
            env: baseEnv,
          }),
        "L1 deployer has pending transactions \\(latest nonce 4, pending nonce 5\\)",
      );
    },
  },
  {
    name: "deployer balance below minimum fails",
    run: async () => {
      await expectRejectsMessage(
        () =>
          runSepoliaPolicyCheck({
            provider: new FakeProvider({ balance: 1n }),
            deployerAddress: "0x1111111111111111111111111111111111111111",
            env: baseEnv,
          }),
        "L1 deployer 0x1111111111111111111111111111111111111111 has 1 wei; fund it to at least 2000000000000000000 wei",
      );
    },
  },
  {
    name: "deploy gas price below current fee fails",
    run: async () => {
      await expectRejectsMessage(
        () =>
          runSepoliaPolicyCheck({
            provider: new FakeProvider({ maxFeePerGas: 6_000_000_000n }),
            deployerAddress: "0x1111111111111111111111111111111111111111",
            env: baseEnv,
          }),
        "L1_DEPLOY_GAS_PRICE_WEI=5000000000 is below current Sepolia execution fee 6000000000",
      );
    },
  },
  {
    name: "blob and finalization caps below current fee fail",
    run: async () => {
      await expectRejectsMessage(
        () =>
          runSepoliaPolicyCheck({
            provider: new FakeProvider({ maxFeePerGas: 101_000_000_000n }),
            deployerAddress: "0x1111111111111111111111111111111111111111",
            env: { ...baseEnv, L1_DEPLOY_GAS_PRICE_WEI: "300000000000" },
          }),
        "L1_BLOB_MAX_FEE_PER_GAS_CAP_WEI=100000000000 is below current Sepolia execution fee 101000000000",
      );

      await expectRejectsMessage(
        () =>
          runSepoliaPolicyCheck({
            provider: new FakeProvider({ maxFeePerGas: 201_000_000_000n }),
            deployerAddress: "0x1111111111111111111111111111111111111111",
            env: {
              ...baseEnv,
              L1_DEPLOY_GAS_PRICE_WEI: "300000000000",
              L1_BLOB_MAX_FEE_PER_GAS_CAP_WEI: "300000000000",
            },
          }),
        "L1_FINALIZATION_MAX_FEE_PER_GAS_CAP_WEI=200000000000 is below current Sepolia execution fee 201000000000",
      );
    },
  },
  {
    name: "blob base fee unavailable warns without leaking URLs",
    run: async () => {
      const report = await runSepoliaPolicyCheck({
        provider: new FakeProvider({
          blobBaseFeeError: new Error("request failed for https://secret.example/rpc"),
        }),
        deployerAddress: "0x1111111111111111111111111111111111111111",
        env: baseEnv,
      });

      assert.equal(report.blobBaseFeeWei, undefined);
      assert.equal(report.warnings.length, 1);
      assert.match(report.warnings[0], /eth_blobBaseFee unavailable/);
      assert.doesNotMatch(report.warnings[0], /secret\.example/);
    },
  },
  {
    name: "invalid decimal wei and boolean values fail clearly",
    run: () => {
      assert.throws(() => parseDecimalWei("BAD_WEI", "12abc"), /BAD_WEI must be an integer wei value/);
      assert.throws(() => parseBoolean("BAD_BOOL", "maybe"), /BAD_BOOL must be true or false/);
    },
  },
  {
    name: "dotenv parsing keeps quotes and comments predictable",
    run: () => {
      assert.deepEqual(readDotEnvContents("A=1\nB='two'\nC=\"three\"\n#D=4\n"), {
        A: "1",
        B: "two",
        C: "three",
      });
    },
  },
  {
    name: "policy defaults match compose and examples",
    run: () => {
      const stackDir = path.resolve(__dirname, "..", "..");
      const compose = fs.readFileSync(path.join(stackDir, "docker-compose.yml"), "utf8");
      const envExample = fs.readFileSync(path.join(stackDir, ".env.example"), "utf8");
      const gasProfile = fs.readFileSync(path.join(stackDir, "profiles/gas-sepolia.env.example"), "utf8");
      const config = buildSepoliaPolicyConfig({});

      for (const [name, value] of Object.entries(SEPOLIA_POLICY_DEFAULTS)) {
        const textValue = value.toString();
        if (name === "SEPOLIA_CHAIN_ID") {
          continue;
        }
        assert.match(compose, new RegExp(`${name}:[^\\n]+\\$\\{${name}:-${textValue}\\}`));
        assert.match(envExample, new RegExp(`# ${name}=${textValue}`));
        assert.match(gasProfile, new RegExp(`${name}=${textValue}`));
      }

      assert.equal(config.l1DeployGasPriceWei, 5_000_000_000n);
      assert.equal(config.l1DynamicGasPriceCapDisabled, true);
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
