import assert from "node:assert/strict";
import { access, mkdtemp, readFile, rm } from "node:fs/promises";
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

test("acknowledges a deployment only after its checkpoint is durable", async () => {
  const directory = await mkdtemp(path.join(tmpdir(), "forge-deployer-process-runner-"));
  const markerFile = path.join(directory, "second-transaction");
  const scriptPath = path.join(__dirname, "fixtures/checkpoint-ack-child.cjs");
  let releaseFirstSave!: () => void;
  let markFirstSaveStarted!: () => void;
  const firstSaveStarted = new Promise<void>((resolve) => {
    markFirstSaveStarted = resolve;
  });
  const firstSaveReleased = new Promise<void>((resolve) => {
    releaseFirstSave = resolve;
  });
  let saveCount = 0;

  const store: CheckpointStore = {
    async load() {
      return undefined;
    },
    async save() {
      saveCount += 1;
      if (saveCount === 1) {
        markFirstSaveStarted();
        await firstSaveReleased;
      }
    },
  };
  const step: DeploymentStep = {
    id: "l1-rollup",
    chain: "l1",
    startingNonce: 0,
    script: "l1-rollup",
    deployments: [
      { key: "l1-rollup.first", contractName: "First", nonce: 0, expectedAddress: FIRST_ADDRESS },
      { key: "l1-rollup.second", contractName: "Second", nonce: 1, expectedAddress: SECOND_ADDRESS },
    ],
  };
  const checkpoint = {
    chainIds: { l1: "31337", l2: "1337" },
    deployments: {},
  } as DeploymentCheckpoint;
  const records = [
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

  try {
    const run = runStepScript({
      scriptPath,
      environment: {
        TEST_DEPLOYMENT_RECORDS: JSON.stringify(records),
        TEST_SECOND_TRANSACTION_MARKER: markerFile,
      },
      step,
      checkpoint,
      store,
      sensitiveValues: [],
    });

    await Promise.race([
      firstSaveStarted,
      run.then(() => {
        throw new Error("child exited before the first checkpoint save started");
      }),
    ]);
    await assert.rejects(access(markerFile), /ENOENT/);

    releaseFirstSave();
    await run;

    assert.equal(await readFile(markerFile, "utf8"), "second transaction submitted");
    assert.equal(saveCount, 2);
  } finally {
    releaseFirstSave();
    await rm(directory, { recursive: true, force: true });
  }
});
