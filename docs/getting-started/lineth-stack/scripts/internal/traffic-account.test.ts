import * as assert from "node:assert/strict";
import * as fs from "node:fs";
import * as os from "node:os";
import * as path from "node:path";

import { Wallet } from "ethers";

import {
  ensureTrafficAccount,
  formatTrafficAccountOutput,
  readRuntimeL2DeployerPrivateKey,
  type TrafficAccountChain,
  type TrafficAccountConfig,
  type TrafficTransactionReceipt,
} from "./traffic-account";

type TestCase = {
  name: string;
  run: () => Promise<void> | void;
};

const DEPLOYER_KEY = `0x${"11".repeat(32)}`;
const ENV_TRAFFIC_KEY = `0x${"22".repeat(32)}`;
const GENERATED_TRAFFIC_KEY = `0x${"33".repeat(32)}`;
const ARTIFACT_TRAFFIC_KEY = `0x${"44".repeat(32)}`;
const WITHDRAW_KEY = `0x${"55".repeat(32)}`;
const TOKEN_ADDRESS = "0x1234567890123456789012345678901234567890";

function addressOf(privateKey: string): string {
  return new Wallet(privateKey).address;
}

function tmpDir(): string {
  return fs.mkdtempSync(path.join(os.tmpdir(), "lineth-traffic-account-"));
}

function writeRuntimeKeys(dir: string, privateKey = DEPLOYER_KEY): string {
  const runtimeKeysPath = path.join(dir, "runtime-keys.env");
  fs.writeFileSync(runtimeKeysPath, `L2_DEPLOYER_PRIVATE_KEY=${privateKey}\n`);
  return runtimeKeysPath;
}

function baseConfig(dir: string, overrides: Partial<TrafficAccountConfig> = {}): TrafficAccountConfig {
  return {
    mode: "ensure",
    env: {},
    runtimeKeysPath: writeRuntimeKeys(dir),
    demoTrafficEnvPath: path.join(dir, "demo-traffic.env"),
    l2GasPriceWei: 100_000_000n,
    ethMinBalanceWei: 100n,
    ethTopUpWei: 1_000n,
    log: () => undefined,
    generatePrivateKey: () => GENERATED_TRAFFIC_KEY,
    ...overrides,
  };
}

function txHash(index: number): string {
  return `0x${index.toString(16).padStart(64, "0")}`;
}

class FakeTrafficChain implements TrafficAccountChain {
  readonly txs: Array<{
    kind: "eth" | "erc20";
    from: string;
    to: string;
    token?: string;
    value: bigint;
    gasPrice: bigint;
  }> = [];

  private readonly ethBalances = new Map<string, bigint>();
  private readonly tokenBalances = new Map<string, bigint>();

  setEthBalance(address: string, balance: bigint) {
    this.ethBalances.set(address.toLowerCase(), balance);
  }

  setTokenBalance(token: string, address: string, balance: bigint) {
    this.tokenBalances.set(`${token.toLowerCase()}:${address.toLowerCase()}`, balance);
  }

  async getEthBalance(address: string): Promise<bigint> {
    return this.ethBalances.get(address.toLowerCase()) ?? 0n;
  }

  async getTokenBalance(token: string, address: string): Promise<bigint> {
    return this.tokenBalances.get(`${token.toLowerCase()}:${address.toLowerCase()}`) ?? 0n;
  }

  async sendEth(params: {
    from: string;
    to: string;
    value: bigint;
    gasPrice: bigint;
  }): Promise<TrafficTransactionReceipt> {
    const fromBalance = await this.getEthBalance(params.from);
    if (fromBalance < params.value) {
      throw new Error(`insufficient fake ETH balance for ${params.from}`);
    }
    this.ethBalances.set(params.from.toLowerCase(), fromBalance - params.value);
    this.ethBalances.set(params.to.toLowerCase(), (await this.getEthBalance(params.to)) + params.value);
    this.txs.push({ kind: "eth", ...params });
    return { hash: txHash(this.txs.length), blockNumber: 100 + this.txs.length };
  }

  async sendToken(params: {
    from: string;
    token: string;
    to: string;
    value: bigint;
    gasPrice: bigint;
  }): Promise<TrafficTransactionReceipt> {
    this.setTokenBalance(params.token, params.to, (await this.getTokenBalance(params.token, params.to)) + params.value);
    this.txs.push({ kind: "erc20", ...params });
    return { hash: txHash(this.txs.length), blockNumber: 100 + this.txs.length };
  }
}

async function expectRejectsMessage(run: () => Promise<unknown>, expected: string) {
  await assert.rejects(run, (error) => {
    assert.ok(error instanceof Error);
    assert.match(error.message, new RegExp(expected));
    return true;
  });
}

const tests: TestCase[] = [
  {
    name: "creates demo-traffic.env atomically when no key exists",
    run: async () => {
      const dir = tmpDir();
      const chain = new FakeTrafficChain();
      chain.setEthBalance(addressOf(DEPLOYER_KEY), 10_000n);

      const result = await ensureTrafficAccount(baseConfig(dir), chain);

      assert.equal(result.address, addressOf(GENERATED_TRAFFIC_KEY));
      assert.equal(result.source, "generated");
      assert.equal(
        fs.readFileSync(path.join(dir, "demo-traffic.env"), "utf8"),
        `L2_TRAFFIC_PRIVATE_KEY=${GENERATED_TRAFFIC_KEY}\n`,
      );
      assert.equal(chain.txs.length, 1);
      assert.equal(chain.txs[0].kind, "eth");
    },
  },
  {
    name: "accepts shell-quoted deployer key from runtime-keys.env",
    run: () => {
      const dir = tmpDir();
      const runtimeKeysPath = path.join(dir, "runtime-keys.env");
      fs.writeFileSync(runtimeKeysPath, `L2_DEPLOYER_PRIVATE_KEY='${DEPLOYER_KEY}'\n`);

      assert.equal(readRuntimeL2DeployerPrivateKey(runtimeKeysPath), DEPLOYER_KEY);
    },
  },
  {
    name: "env-provided traffic key wins and is never persisted",
    run: async () => {
      const dir = tmpDir();
      const chain = new FakeTrafficChain();
      chain.setEthBalance(addressOf(DEPLOYER_KEY), 10_000n);

      const result = await ensureTrafficAccount(
        baseConfig(dir, {
          env: { L2_TRAFFIC_PRIVATE_KEY: ENV_TRAFFIC_KEY },
        }),
        chain,
      );

      assert.equal(result.address, addressOf(ENV_TRAFFIC_KEY));
      assert.equal(result.source, "env");
      assert.equal(fs.existsSync(path.join(dir, "demo-traffic.env")), false);
    },
  },
  {
    name: "existing demo-traffic.env is reused",
    run: async () => {
      const dir = tmpDir();
      fs.writeFileSync(path.join(dir, "demo-traffic.env"), `L2_TRAFFIC_PRIVATE_KEY=${ARTIFACT_TRAFFIC_KEY}\n`);
      const chain = new FakeTrafficChain();
      chain.setEthBalance(addressOf(DEPLOYER_KEY), 10_000n);

      const result = await ensureTrafficAccount(baseConfig(dir), chain);

      assert.equal(result.address, addressOf(ARTIFACT_TRAFFIC_KEY));
      assert.equal(result.source, "artifact");
    },
  },
  {
    name: "require-existing fails when no existing key is available",
    run: async () => {
      const dir = tmpDir();
      const chain = new FakeTrafficChain();
      chain.setEthBalance(addressOf(DEPLOYER_KEY), 10_000n);

      await expectRejectsMessage(
        () => ensureTrafficAccount(baseConfig(dir, { mode: "require-existing" }), chain),
        "no disposable traffic account found",
      );
    },
  },
  {
    name: "require-existing accepts L2_WITHDRAW_PRIVATE_KEY without persistence",
    run: async () => {
      const dir = tmpDir();
      const chain = new FakeTrafficChain();
      chain.setEthBalance(addressOf(DEPLOYER_KEY), 10_000n);

      const result = await ensureTrafficAccount(
        baseConfig(dir, {
          mode: "require-existing",
          env: { L2_WITHDRAW_PRIVATE_KEY: WITHDRAW_KEY },
        }),
        chain,
      );

      assert.equal(result.address, addressOf(WITHDRAW_KEY));
      assert.equal(result.source, "env");
      assert.equal(fs.existsSync(path.join(dir, "demo-traffic.env")), false);
    },
  },
  {
    name: "skips ETH top-up when balance is already above the minimum",
    run: async () => {
      const dir = tmpDir();
      const chain = new FakeTrafficChain();
      chain.setEthBalance(addressOf(DEPLOYER_KEY), 10_000n);
      chain.setEthBalance(addressOf(GENERATED_TRAFFIC_KEY), 500n);

      const result = await ensureTrafficAccount(baseConfig(dir), chain);

      assert.equal(result.ethTopUpTx, undefined);
      assert.equal(chain.txs.length, 0);
    },
  },
  {
    name: "tops up ERC20 only when token config is provided and balance is below minimum",
    run: async () => {
      const dir = tmpDir();
      const chain = new FakeTrafficChain();
      chain.setEthBalance(addressOf(DEPLOYER_KEY), 10_000n);
      chain.setEthBalance(addressOf(GENERATED_TRAFFIC_KEY), 500n);
      chain.setTokenBalance(TOKEN_ADDRESS, addressOf(GENERATED_TRAFFIC_KEY), 5n);

      const result = await ensureTrafficAccount(
        baseConfig(dir, {
          erc20: {
            tokenAddress: TOKEN_ADDRESS,
            minBalanceWei: 10n,
            topUpWei: 20n,
          },
        }),
        chain,
      );

      assert.equal(result.erc20BalanceWei, 25n);
      assert.equal(result.erc20TopUpTx?.hash, txHash(1));
      assert.deepEqual(
        chain.txs.map((tx) => tx.kind),
        ["erc20"],
      );
    },
  },
  {
    name: "parseable output and logs never contain private keys",
    run: async () => {
      const dir = tmpDir();
      const logs: string[] = [];
      const chain = new FakeTrafficChain();
      chain.setEthBalance(addressOf(DEPLOYER_KEY), 10_000n);

      const result = await ensureTrafficAccount(baseConfig(dir, { log: (line) => logs.push(line) }), chain);
      const output = formatTrafficAccountOutput(result);

      assert.match(output, /^TRAFFIC_ACCOUNT_ADDRESS=0x[a-fA-F0-9]{40}$/m);
      assert.match(output, /^TRAFFIC_ACCOUNT_SOURCE=generated$/m);
      for (const line of [...logs, output]) {
        assert.doesNotMatch(line, new RegExp(GENERATED_TRAFFIC_KEY.slice(2), "i"));
        assert.doesNotMatch(line, new RegExp(DEPLOYER_KEY.slice(2), "i"));
      }
    },
  },
  {
    name: "invalid private key and invalid ERC20 address fail clearly",
    run: async () => {
      const dir = tmpDir();
      const chain = new FakeTrafficChain();
      chain.setEthBalance(addressOf(DEPLOYER_KEY), 10_000n);

      await expectRejectsMessage(
        () => ensureTrafficAccount(baseConfig(dir, { env: { L2_TRAFFIC_PRIVATE_KEY: "0x1234" } }), chain),
        "L2_TRAFFIC_PRIVATE_KEY is malformed",
      );

      await expectRejectsMessage(
        () =>
          ensureTrafficAccount(
            baseConfig(dir, {
              erc20: { tokenAddress: "0x1234", minBalanceWei: 1n, topUpWei: 1n },
            }),
            chain,
          ),
        "TRAFFIC_ERC20_ADDRESS is invalid",
      );
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
  console.error(error);
  process.exit(1);
});
