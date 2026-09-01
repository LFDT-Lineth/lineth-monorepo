import assert from "node:assert/strict";
import { access, mkdtemp, readdir, readFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import test from "node:test";

import { DeploymentStep } from "../src/address-plan";
import { DeploymentCheckpoint } from "../src/checkpoint";
import { childEnvironment, runStepScript } from "../src/process-runner";
import { CheckpointStore } from "../src/store";

const FIRST_ADDRESS = "0x1000000000000000000000000000000000000001";
const SECOND_ADDRESS = "0x2000000000000000000000000000000000000002";
const FIRST_HASH = `0x${"11".repeat(32)}`;
const SECOND_HASH = `0x${"22".repeat(32)}`;

const STEP: DeploymentStep = {
  id: "l1-rollup",
  chain: "l1",
  startingNonce: 0,
  script: "l1-rollup",
  deployments: [
    { key: "l1-rollup.first", contractName: "First", nonce: 0, expectedAddress: FIRST_ADDRESS },
    { key: "l1-rollup.second", contractName: "Second", nonce: 1, expectedAddress: SECOND_ADDRESS },
  ],
};

const RECORDS = [
  {
    contractName: "First",
    address: FIRST_ADDRESS,
    transactionHash: FIRST_HASH,
    blockNumber: 1,
    chainId: "31337",
  },
  {
    contractName: "Second",
    address: SECOND_ADDRESS,
    transactionHash: SECOND_HASH,
    blockNumber: 2,
    chainId: "31337",
  },
];

function checkpoint(): DeploymentCheckpoint {
  return {
    chainIds: { l1: "31337", l2: "1337" },
    deployments: {},
    inFlightDeployments: {},
    bootstrap: {},
    inFlightBootstrap: {},
  } as DeploymentCheckpoint;
}

function controlledStore(options: { blockAt?: number; rejectAt?: number } = {}) {
  let releaseBlockedSave = () => {};
  let markBlockedSaveStarted = () => {};
  const blockedSaveStarted = new Promise<void>((resolve) => {
    markBlockedSaveStarted = resolve;
  });
  const blockedSaveReleased = new Promise<void>((resolve) => {
    releaseBlockedSave = resolve;
  });
  const durableSnapshots: DeploymentCheckpoint[] = [];
  let saveCount = 0;

  const store: CheckpointStore = {
    async load() {
      return undefined;
    },
    async save(value) {
      saveCount += 1;
      if (saveCount === options.blockAt) {
        markBlockedSaveStarted();
        await blockedSaveReleased;
      }
      if (saveCount === options.rejectAt) throw new Error(`save ${saveCount} rejected`);
      durableSnapshots.push(structuredClone(value));
    },
  };

  return {
    store,
    blockedSaveStarted,
    releaseBlockedSave,
    durableSnapshots,
    saveCount: () => saveCount,
  };
}

async function fixtureRun(store: CheckpointStore, value = checkpoint()) {
  const directory = await mkdtemp(path.join(tmpdir(), "forge-deployer-process-runner-"));
  const firstMarkerFile = path.join(directory, "first-broadcast");
  const secondMarkerFile = path.join(directory, "second-transaction");
  const run = runStepScript({
    scriptPath: path.join(__dirname, "fixtures/checkpoint-ack-child.cjs"),
    environment: {
      TEST_DEPLOYMENT_RECORDS: JSON.stringify(RECORDS),
      TEST_FIRST_BROADCAST_MARKER: firstMarkerFile,
      TEST_SECOND_TRANSACTION_MARKER: secondMarkerFile,
      TEST_ACK_TIMEOUT_MS: "5000",
    },
    step: STEP,
    checkpoint: value,
    store,
    sensitiveValues: [],
  });
  return { directory, firstMarkerFile, secondMarkerFile, run, checkpoint: value };
}

test("does not inherit unvalidated deployment overrides", () => {
  const environment = childEnvironment(
    { RPC_URL: "http://l1.example" },
    {
      PATH: "/usr/bin",
      NODE_EXTRA_CA_CERTS: "/etc/private-ca.pem",
      LINETH_ROLLUP_ROLE_ADDRESSES: "malicious-unvalidated-value",
      NODE_OPTIONS: "--require=/tmp/untrusted.js",
    },
  );

  assert.deepEqual(environment, {
    NODE_EXTRA_CA_CERTS: "/etc/private-ca.pem",
    PATH: "/usr/bin",
    RPC_URL: "http://l1.example",
  });
});

test("blocked intent persistence prevents the first broadcast", async () => {
  const control = controlledStore({ blockAt: 1 });
  const fixture = await fixtureRun(control.store);
  try {
    await Promise.race([
      control.blockedSaveStarted,
      fixture.run.then(() => {
        throw new Error("child exited before the first checkpoint save started");
      }),
    ]);
    await new Promise((resolve) => setTimeout(resolve, 100));
    assert.deepEqual(await readdir(fixture.directory), []);
    assert.equal(control.durableSnapshots.length, 0);

    control.releaseBlockedSave();
    await fixture.run;

    assert.equal(await readFile(fixture.firstMarkerFile, "utf8"), "first transaction submitted");
    assert.equal(control.saveCount(), 4);
  } finally {
    control.releaseBlockedSave();
    await rm(fixture.directory, { recursive: true, force: true });
  }
});

test("rejected intent persistence aborts before the first broadcast", async () => {
  const control = controlledStore({ rejectAt: 1 });
  const fixture = await fixtureRun(control.store);
  try {
    await assert.rejects(fixture.run, /save 1 rejected/);
    await assert.rejects(access(fixture.firstMarkerFile), /ENOENT/);
    assert.equal(control.saveCount(), 1);
    assert.equal(control.durableSnapshots.length, 0);
  } finally {
    await rm(fixture.directory, { recursive: true, force: true });
  }
});

test("a durable intent permits the first broadcast but blocked receipt persistence prevents the second", async () => {
  const control = controlledStore({ blockAt: 2 });
  const fixture = await fixtureRun(control.store);
  try {
    await control.blockedSaveStarted;
    assert.equal(await readFile(fixture.firstMarkerFile, "utf8"), "first transaction submitted");
    await new Promise((resolve) => setTimeout(resolve, 100));
    await assert.rejects(access(fixture.secondMarkerFile), /ENOENT/);
    assert.deepEqual(Object.keys(control.durableSnapshots[0]!.inFlightDeployments), ["l1-rollup.first"]);
    assert.deepEqual(control.durableSnapshots[0]!.deployments, {});

    control.releaseBlockedSave();
    await fixture.run;
  } finally {
    control.releaseBlockedSave();
    await rm(fixture.directory, { recursive: true, force: true });
  }
});

test("rejected receipt persistence aborts before the second transaction", async () => {
  const control = controlledStore({ rejectAt: 2 });
  const fixture = await fixtureRun(control.store);
  try {
    await assert.rejects(fixture.run, /save 2 rejected/);
    assert.equal(await readFile(fixture.firstMarkerFile, "utf8"), "first transaction submitted");
    await assert.rejects(access(fixture.secondMarkerFile), /ENOENT/);
    assert.equal(control.durableSnapshots.length, 1);
    assert.deepEqual(Object.keys(control.durableSnapshots[0]!.inFlightDeployments), ["l1-rollup.first"]);
    assert.deepEqual(control.durableSnapshots[0]!.deployments, {});
  } finally {
    await rm(fixture.directory, { recursive: true, force: true });
  }
});

test("durable receipts atomically replace intents and allow the next transaction", async () => {
  const control = controlledStore();
  const fixture = await fixtureRun(control.store);
  try {
    await fixture.run;

    assert.equal(await readFile(fixture.secondMarkerFile, "utf8"), "second transaction submitted");
    assert.equal(control.saveCount(), 4);
    assert.deepEqual(Object.keys(control.durableSnapshots[0]!.inFlightDeployments), ["l1-rollup.first"]);
    assert.deepEqual(control.durableSnapshots[1]!.inFlightDeployments, {});
    assert.ok(control.durableSnapshots[1]!.deployments["l1-rollup.first"]);
    assert.deepEqual(Object.keys(control.durableSnapshots[2]!.inFlightDeployments), ["l1-rollup.second"]);
    assert.deepEqual(control.durableSnapshots[3]!.inFlightDeployments, {});
    assert.ok(control.durableSnapshots[3]!.deployments["l1-rollup.second"]);
  } finally {
    await rm(fixture.directory, { recursive: true, force: true });
  }
});

test("redacts credential canaries from child stdout and stderr", async () => {
  const keyCanary = "__PRIVATE_KEY_CANARY__";
  const rpcCanary = "user:password@rpc.example";
  const output: string[] = [];
  const originalLog = console.log;
  const originalError = console.error;
  console.log = (...values: unknown[]) => output.push(values.join(" "));
  console.error = (...values: unknown[]) => output.push(values.join(" "));

  try {
    await runStepScript({
      scriptPath: path.join(__dirname, "fixtures/log-canary-child.cjs"),
      environment: { TEST_KEY_CANARY: keyCanary, TEST_RPC_CANARY: rpcCanary },
      step: { id: "l1-rollup", chain: "l1", startingNonce: 0, script: "l1-rollup", deployments: [] },
      checkpoint: checkpoint(),
      store: controlledStore().store,
      sensitiveValues: [keyCanary, rpcCanary],
    });
  } finally {
    console.log = originalLog;
    console.error = originalError;
  }

  assert.doesNotMatch(output.join("\n"), /__PRIVATE_KEY_CANARY__|user:password@rpc\.example/);
  assert.match(output.join("\n"), /\[REDACTED\]/);
});
