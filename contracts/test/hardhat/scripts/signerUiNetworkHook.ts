import { expect } from "chai";
import hre from "hardhat";
import { createHardhatRuntimeEnvironment } from "hardhat/hre";
import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { after, afterEach, before, describe, it } from "node:test";
import { type Address, type Hash, getAddress, hexToBigInt, parseEther } from "viem";
import { generatePrivateKey, privateKeyToAccount } from "viem/accounts";

import { __testOnlySignerUiBridge } from "../../../hardhat-plugins/signer-ui/bridge.js";
import {
  getSignerUiBroadcastLogPath,
  readSignerUiBroadcasts,
} from "../../../hardhat-plugins/signer-ui/broadcast-log.js";
import {
  getUiWalletClient,
  setUiTransactionContext,
  withSignerUi,
} from "../../../hardhat-plugins/signer-ui/session.js";
import hardhatConfig from "../../../hardhat.config.js";

import type {
  SerializedTransactionRequest,
  SessionWalletState,
  SignerUiSessionContext,
  SignerUiSessionHandle,
  TransactionOutcome,
  UiTransactionDetails,
} from "../../../hardhat-plugins/signer-ui/bridge.js";
import type { HardhatRuntimeEnvironment } from "hardhat/types/hre";
import type { NetworkConnection } from "hardhat/types/network";

type RecordedRequest = {
  request: SerializedTransactionRequest;
  label: string;
  description: string;
  details: UiTransactionDetails | undefined;
};

/**
 * Stands in for the browser: "signs" with a key Hardhat does not know and broadcasts the raw transaction through the
 * same connection, like a wallet sending to the RPC the UI targets.
 */
class StubSignerUiSession implements SignerUiSessionHandle {
  public readonly requests: RecordedRequest[] = [];
  public readonly scriptContexts: string[] = [];
  public started = 0;
  public closed = 0;
  public rejectNextWith: Error | undefined;

  public constructor(
    private readonly context: SignerUiSessionContext,
    private readonly connection: NetworkConnection,
    private readonly account: ReturnType<typeof privateKeyToAccount>,
  ) {}

  public get chainId(): number {
    return this.context.chain.chainId;
  }

  public async start(): Promise<void> {
    this.started += 1;
  }

  public async close(): Promise<void> {
    this.closed += 1;
  }

  public notifyScriptContext(scriptContext: string): void {
    this.scriptContexts.push(scriptContext);
  }

  public async waitForWallet(): Promise<SessionWalletState> {
    return { address: this.account.address, chainId: this.chainId, connectedAt: new Date().toISOString() };
  }

  public async requestTransaction(
    request: SerializedTransactionRequest,
    label: string,
    description: string,
    details: UiTransactionDetails | undefined,
  ): Promise<TransactionOutcome> {
    this.requests.push({ request, label, description, details });
    const rejection = this.rejectNextWith;
    if (rejection !== undefined) {
      this.rejectNextWith = undefined;
      throw rejection;
    }

    const publicClient = await this.connection.viem.getPublicClient();
    const to = request.to === undefined ? null : getAddress(request.to);
    const value = request.value === undefined ? 0n : hexToBigInt(request.value);
    const nonce = await publicClient.getTransactionCount({ address: this.account.address, blockTag: "pending" });
    const gas =
      request.gas === undefined
        ? await publicClient.estimateGas({
            account: this.account.address,
            ...(to === null ? {} : { to }),
            ...(request.data === undefined ? {} : { data: request.data }),
            value,
          })
        : hexToBigInt(request.gas);
    const fees = await publicClient.estimateFeesPerGas();
    const signed = await this.account.signTransaction({
      type: "eip1559",
      chainId: this.chainId,
      nonce,
      gas,
      to,
      ...(request.data === undefined ? {} : { data: request.data }),
      value,
      maxFeePerGas: request.maxFeePerGas === undefined ? fees.maxFeePerGas : hexToBigInt(request.maxFeePerGas),
      maxPriorityFeePerGas:
        request.maxPriorityFeePerGas === undefined
          ? fees.maxPriorityFeePerGas
          : hexToBigInt(request.maxPriorityFeePerGas),
    });
    const hash: Hash = await publicClient.sendRawTransaction({ serializedTransaction: signed });
    return {
      requestId: `stub-${this.requests.length}`,
      hash,
      from: this.account.address,
      chainId: this.chainId,
      nonce,
    };
  }

  public clearTransactionProgress(): void {}

  public setWorkflowStatus(): void {}

  public clearWorkflowStatus(): void {}

  public setTerminalSessionOutcome(): void {}
}

const ENV_KEYS = [
  "HARDHAT_SIGNER_UI",
  "DEPLOYER_PRIVATE_KEY",
  "EXPECTED_SIGNER_ADDRESS",
  "HARDHAT_SIGNER_UI_SHUTDOWN_DRAIN_MS",
] as const;

/** `withSignerUi` scopes: the network hook is registered only inside them, on an HRE started with `--signer-ui`. */
describe("signer-ui withSignerUi scopes", () => {
  const browserAccount = privateKeyToAccount(generatePrivateKey());
  const savedEnv = new Map<string, string | undefined>();
  const sessions: StubSignerUiSession[] = [];
  let uiHre: HardhatRuntimeEnvironment;
  let connection: NetworkConnection;
  let logDir: string;

  const lastSession = (): StubSignerUiSession => {
    const session = sessions.at(-1);
    if (session === undefined) {
      throw new Error("no signer UI session was opened");
    }
    return session;
  };
  const inScope = <T>(label: string, fn: () => Promise<T>) => withSignerUi(uiHre, label, fn);
  const networkAccounts = () => connection.provider.request({ method: "eth_accounts" });

  before(async () => {
    for (const key of ENV_KEYS) {
      savedEnv.set(key, process.env[key]);
      delete process.env[key];
    }
    process.env.HARDHAT_SIGNER_UI_SHUTDOWN_DRAIN_MS = "0";
    uiHre = await createHardhatRuntimeEnvironment(hardhatConfig, { signerUi: true });
    connection = await uiHre.network.getOrCreate();
    logDir = mkdtempSync(join(tmpdir(), "signer-ui-hook-"));
    __testOnlySignerUiBridge.setBroadcastLogDir(logDir);
    __testOnlySignerUiBridge.setSessionFactory((context) => {
      const session = new StubSignerUiSession(context, connection, browserAccount);
      sessions.push(session);
      return session;
    });
    const testClient = await connection.viem.getTestClient();
    await testClient.setBalance({ address: browserAccount.address, value: parseEther("10") });
  });

  afterEach(async () => {
    await __testOnlySignerUiBridge.closeActiveSession();
  });

  after(() => {
    __testOnlySignerUiBridge.setSessionFactory(undefined);
    __testOnlySignerUiBridge.setBroadcastLogDir(undefined);
    rmSync(logDir, { recursive: true, force: true });
    for (const [key, value] of savedEnv) {
      if (value === undefined) {
        delete process.env[key];
      } else {
        process.env[key] = value;
      }
    }
  });

  it("leaves eth_accounts to the network outside a scope, and without --signer-ui", async () => {
    expect(await networkAccounts()).to.not.include(browserAccount.address.toLowerCase());

    const withoutFlag = await withSignerUi(hre, "no flag", async () => {
      const defaultConnection = await hre.network.getOrCreate();
      return defaultConnection.provider.request({ method: "eth_accounts" });
    });
    expect(withoutFlag).to.be.an("array").that.is.not.empty;
    expect(withoutFlag).to.not.include(browserAccount.address.toLowerCase());
    expect(__testOnlySignerUiBridge.getActiveSession()).to.equal(undefined);
  });

  it("answers eth_accounts and eth_requestAccounts with the browser wallet, opening one session, closed at the end", async () => {
    const sessionsBefore = sessions.length;

    const [accounts, requested] = await inScope("accounts test", () =>
      Promise.all([
        connection.provider.request({ method: "eth_accounts" }),
        connection.provider.request({ method: "eth_requestAccounts" }),
      ]),
    );

    expect(accounts).to.deep.equal([browserAccount.address.toLowerCase()]);
    expect(requested).to.deep.equal([browserAccount.address.toLowerCase()]);
    expect(sessions.length).to.equal(sessionsBefore + 1);
    expect(lastSession().started).to.equal(1);
    expect(lastSession().closed, "the scope closes its session").to.equal(1);
    expect(__testOnlySignerUiBridge.getActiveSession()).to.equal(undefined);
  });

  it("passes other methods through to the network without opening a session", async () => {
    const sessionsBefore = sessions.length;

    const [chainId, balance] = await inScope("pass-through test", () =>
      Promise.all([
        connection.provider.request({ method: "eth_chainId" }),
        connection.provider.request({ method: "eth_getBalance", params: [browserAccount.address, "latest"] }),
      ]),
    );

    expect(chainId).to.equal("0x7a69");
    expect(balance).to.not.equal("0x0");
    expect(sessions.length).to.equal(sessionsBefore);
  });

  it("routes eth_sendTransaction to the session, returns its hash and appends the broadcast log", async () => {
    const recipient: Address = privateKeyToAccount(generatePrivateKey()).address;

    const hash = await inScope("hook routing test", async () => {
      const wallet = await getUiWalletClient(connection);
      expect(wallet.account.address).to.equal(browserAccount.address);

      setUiTransactionContext({ contractName: "Transfer", notes: "routing test" });
      return wallet.sendTransaction({ to: recipient, value: 1234n });
    });

    const session = lastSession();
    expect(session.scriptContexts).to.deep.equal(["hook routing test"]);
    expect(session.closed, "the scope closes its session").to.equal(1);
    expect(session.requests).to.have.length(1);
    const [recorded] = session.requests;
    expect(recorded?.label).to.equal("Contract transaction");
    expect(recorded?.request.to).to.equal(recipient);
    expect(recorded?.request.value).to.equal("0x4d2");
    expect(recorded?.request.chainId).to.equal("0x7a69");
    expect(recorded?.details).to.deep.equal({ contractName: "Transfer", notes: "routing test" });

    const publicClient = await connection.viem.getPublicClient();
    const receipt = await publicClient.waitForTransactionReceipt({ hash });
    expect(receipt.status).to.equal("success");
    expect(getAddress(receipt.from)).to.equal(browserAccount.address);
    expect(await publicClient.getBalance({ address: recipient })).to.equal(1234n);

    const transaction = await publicClient.getTransaction({ hash });
    const log = readSignerUiBroadcasts(getSignerUiBroadcastLogPath(logDir, 31337));
    expect(log.at(-1)).to.deep.include({ sender: browserAccount.address, nonce: transaction.nonce, hash });
  });

  it("labels contract creations routed through the hook", async () => {
    const contract = await inScope("creation test", async () => {
      const wallet = await getUiWalletClient(connection);
      return connection.viem.deployContract("TestEfficientLeftRightKeccak", [], { client: { wallet } });
    });

    const [recorded] = lastSession().requests;
    expect(recorded?.label).to.equal("Contract creation");
    expect(recorded?.request.to).to.equal(undefined);
    const publicClient = await connection.viem.getPublicClient();
    expect(await publicClient.getCode({ address: contract.address })).to.not.equal(undefined);
    const log = readSignerUiBroadcasts(getSignerUiBroadcastLogPath(logDir, 31337));
    expect(log.map((entry) => entry.sender)).to.include(browserAccount.address);
  });

  it("returns a 4001 error and logs nothing when the browser wallet rejects", async () => {
    const logPath = getSignerUiBroadcastLogPath(logDir, 31337);
    const entriesBefore = readSignerUiBroadcasts(logPath).length;

    const caught = await inScope("rejection test", async () => {
      await connection.provider.request({ method: "eth_accounts" });
      lastSession().rejectNextWith = new __testOnlySignerUiBridge.SignerUiWalletError("User rejected the request.");
      try {
        await connection.provider.request({
          method: "eth_sendTransaction",
          params: [{ from: browserAccount.address, to: browserAccount.address, value: "0x1" }],
        });
      } catch (error) {
        return error;
      }
      return undefined;
    });

    expect(caught).to.be.instanceOf(Error);
    expect(caught).to.have.property("code", 4001);
    expect(String(caught)).to.contain("User rejected the request.");
    expect(readSignerUiBroadcasts(logPath)).to.have.length(entriesBefore);
  });

  it("refuses transactions from another account and signing methods the bridge cannot serve", async () => {
    const otherAccount = privateKeyToAccount(generatePrivateKey()).address;

    const [fromMismatch, signError] = await inScope("refusal test", async () => {
      const errors: unknown[] = [];
      for (const request of [
        { method: "eth_sendTransaction", params: [{ from: otherAccount, to: browserAccount.address, value: "0x1" }] },
        { method: "personal_sign", params: ["0x00", browserAccount.address] },
      ]) {
        try {
          await connection.provider.request(request);
          errors.push(undefined);
        } catch (error) {
          errors.push(error);
        }
      }
      return errors;
    });
    expect(fromMismatch).to.have.property("code", 4100);
    expect(signError).to.have.property("code", 4200);
  });

  it("unregisters the hook and closes the session when the scope throws", async () => {
    let thrown: unknown;
    try {
      await inScope("failing scope", async () => {
        await connection.provider.request({ method: "eth_accounts" });
        throw new Error("deploy failed");
      });
    } catch (error) {
      thrown = error;
    }

    expect(thrown).to.have.property("message", "deploy failed");
    expect(lastSession().closed).to.equal(1);
    expect(__testOnlySignerUiBridge.getActiveSession()).to.equal(undefined);
    expect(await networkAccounts()).to.not.include(browserAccount.address.toLowerCase());
  });

  it("refuses to open a scope while DEPLOYER_PRIVATE_KEY is also set", async () => {
    process.env.DEPLOYER_PRIVATE_KEY = "0x1";
    try {
      let thrown: unknown;
      try {
        await inScope("conflict", async () => undefined);
      } catch (error) {
        thrown = error;
      }
      expect(String(thrown)).to.contain("DEPLOYER_PRIVATE_KEY are both set");
    } finally {
      delete process.env.DEPLOYER_PRIVATE_KEY;
    }
  });
});
