import assert from "node:assert/strict";
import test from "node:test";

import { buildAddressPlan } from "../src/address-plan";
import {
  assertCheckpointCompatible,
  assertNoInFlightBootstrapItems,
  assertNoInFlightDeployments,
  createCheckpoint,
  DEPLOYMENT_PROFILE,
  deploymentConfigurationHash,
  parseCheckpoint,
  SCHEMA_VERSION,
} from "../src/checkpoint";
import {
  checkpointToConfigMapData,
  configMapContainsData,
  configMapDataPatch,
  DEFAULT_CONFIG_MAP_NAME,
  isKubernetesConflictStatus,
  isRetryableKubernetesStatus,
} from "../src/store";

const STATE_ROOT = `0x${"ab".repeat(32)}`;
const IMAGE_DIGEST = `sha256:${"cd".repeat(32)}`;
const L1_SIGNER = "0x1000000000000000000000000000000000000001";
const L2_SIGNER = "0x2000000000000000000000000000000000000002";

function identity() {
  return {
    profile: DEPLOYMENT_PROFILE,
    schemaVersion: SCHEMA_VERSION,
    artifactDigest: IMAGE_DIGEST,
    initialL2StateRootHash: STATE_ROOT,
    l2GenesisTimestamp: 1_700_000_000,
    chainIds: { l1: "1", l2: "1337" },
    signers: { l1: L1_SIGNER, l2: L2_SIGNER },
    configurationHash: `0x${"12".repeat(32)}`,
    bootstrapHash: `0x${"00".repeat(32)}`,
    startingNonces: { l1: 3, l2: 7 },
  };
}

function checkpoint() {
  const startingNonces = identity().startingNonces;
  return createCheckpoint({
    ...identity(),
    startingNonces,
    plan: buildAddressPlan({
      l1Signer: L1_SIGNER,
      l2Signer: L2_SIGNER,
      l1StartingNonce: startingNonces.l1,
      l2StartingNonce: startingNonces.l2,
    }),
  });
}

test("creates a versioned checkpoint with every expected address and no secrets", () => {
  const value = checkpoint();
  const serialized = JSON.stringify(value);
  const startingNonces = identity().startingNonces;
  const expectedDeploymentCount = buildAddressPlan({
    l1Signer: L1_SIGNER,
    l2Signer: L2_SIGNER,
    l1StartingNonce: startingNonces.l1,
    l2StartingNonce: startingNonces.l2,
  }).reduce((total, step) => total + step.deployments.length, 0);

  assert.equal(value.profile, DEPLOYMENT_PROFILE);
  assert.equal(SCHEMA_VERSION, 4);
  assert.equal(value.schemaVersion, 4);
  assert.equal(value.artifactDigest, IMAGE_DIGEST);
  assert.deepEqual(value.inFlightDeployments, {});
  assert.deepEqual(value.inFlightBootstrap, {});
  assert.equal(Object.keys(value.expectedDeployments).length, expectedDeploymentCount);
  assert.doesNotMatch(serialized, /private.?key|rpc.?url/i);
});

test("rejects incompatible chain, signer, state root, profile, and artifact", () => {
  const value = checkpoint();

  assert.throws(
    () => assertCheckpointCompatible(value, { ...identity(), chainIds: { l1: "11155111", l2: "1337" } }),
    /L1 chain ID/,
  );
  assert.throws(
    () =>
      assertCheckpointCompatible(value, {
        ...identity(),
        signers: { l1: "0x3000000000000000000000000000000000000003", l2: L2_SIGNER },
      }),
    /L1 signer/,
  );
  assert.throws(
    () => assertCheckpointCompatible(value, { ...identity(), initialL2StateRootHash: `0x${"cd".repeat(32)}` }),
    /initial L2 state root/,
  );
  assert.throws(
    () => assertCheckpointCompatible(value, { ...identity(), profile: "another-profile" }),
    /deployment profile/,
  );
  assert.throws(
    () => assertCheckpointCompatible(value, { ...identity(), configurationHash: `0x${"34".repeat(32)}` }),
    /deployment configuration/,
  );
  assert.throws(
    () => assertCheckpointCompatible(value, { ...identity(), startingNonces: { l1: 4, l2: 7 } }),
    /L1 starting nonce/,
  );
  assert.throws(
    () => assertCheckpointCompatible(value, { ...identity(), startingNonces: { l1: 3, l2: 8 } }),
    /L2 starting nonce/,
  );
  assert.throws(
    () => assertCheckpointCompatible(value, { ...identity(), artifactDigest: `sha256:${"ef".repeat(32)}` }),
    /artifact digest/,
  );
  assert.throws(
    () => assertCheckpointCompatible(value, { ...identity(), bootstrapHash: `0x${"11".repeat(32)}` }),
    /bootstrap manifest/,
  );
});

test("round-trips completed bootstrap records and rejects malformed ones", () => {
  const value = checkpoint();
  value.bootstrap["bootstrap.fund-relayer"] = {
    kind: "sign",
    chain: "l2",
    transactionHash: `0x${"cd".repeat(32)}`,
    blockNumber: 42,
    chainId: "1337",
  };
  value.bootstrap["bootstrap.deploy-factory"] = {
    kind: "presigned",
    chain: "l2",
    transactionHash: `0x${"ef".repeat(32)}`,
    blockNumber: 43,
    chainId: "1337",
    address: "0x4e59b44847b379578588920cA78FbF26c0B4956C",
  };
  assert.deepEqual(parseCheckpoint(JSON.stringify(value)).bootstrap, value.bootstrap);

  const malformed = checkpoint();
  malformed.bootstrap["bootstrap.bad"] = {
    kind: "sign",
    chain: "l2",
    transactionHash: "not-a-hash",
    blockNumber: 1,
    chainId: "1337",
  };
  assert.throws(() => parseCheckpoint(JSON.stringify(malformed)), /invalid bootstrap record/);

  const missing = checkpoint() as Partial<ReturnType<typeof checkpoint>>;
  delete (missing as Record<string, unknown>).bootstrap;
  assert.throws(() => parseCheckpoint(JSON.stringify(missing)), /missing bootstrap record/);
});

test("round-trips in-flight bootstrap intents and rejects malformed ones", () => {
  const value = checkpoint();
  value.inFlightBootstrap["bootstrap.fund-relayer"] = { kind: "sign", chain: "l2", nonce: 12 };
  value.inFlightBootstrap["bootstrap.deploy-factory"] = { kind: "presigned", chain: "l2" };
  value.inFlightBootstrap["bootstrap.run-migration"] = { kind: "script", chain: "l1" };
  assert.deepEqual(parseCheckpoint(JSON.stringify(value)).inFlightBootstrap, value.inFlightBootstrap);

  const mutations: Array<[label: string, intent: Record<string, unknown>]> = [
    ["bad kind", { kind: "transfer", chain: "l2" }],
    ["bad chain", { kind: "sign", chain: "l3" }],
    ["negative nonce", { kind: "sign", chain: "l2", nonce: -1 }],
    ["unsafe nonce", { kind: "sign", chain: "l2", nonce: Number.MAX_SAFE_INTEGER + 1 }],
  ];
  for (const [label, intent] of mutations) {
    const malformed = checkpoint();
    malformed.inFlightBootstrap["bootstrap.bad"] = intent as never;
    assert.throws(() => parseCheckpoint(JSON.stringify(malformed)), /invalid in-flight bootstrap item/, label);
  }

  const missing = checkpoint() as Partial<ReturnType<typeof checkpoint>>;
  delete (missing as Record<string, unknown>).inFlightBootstrap;
  assert.throws(() => parseCheckpoint(JSON.stringify(missing)), /missing in-flight bootstrap record/);
});

test("fails closed when a bootstrap item broadcast may be unresolved", () => {
  const value = checkpoint();
  assert.doesNotThrow(() => assertNoInFlightBootstrapItems(value));

  value.inFlightBootstrap["bootstrap.fund-relayer"] = { kind: "sign", chain: "l2", nonce: 12 };
  assert.throws(
    () => assertNoInFlightBootstrapItems(value),
    /checkpoint contains unresolved in-flight bootstrap item bootstrap\.fund-relayer at nonce 12/,
  );

  const nonceless = checkpoint();
  nonceless.inFlightBootstrap["bootstrap.run-migration"] = { kind: "script", chain: "l1" };
  assert.throws(
    () => assertNoInFlightBootstrapItems(nonceless),
    /checkpoint contains unresolved in-flight bootstrap item bootstrap\.run-migration; transaction broadcast is ambiguous/,
  );
});

test("serializes sufficient ConfigMap state and public addresses", () => {
  const value = checkpoint();
  const data = checkpointToConfigMapData(value);
  const restored = JSON.parse(data["checkpoint.json"] ?? "");

  assert.equal(DEFAULT_CONFIG_MAP_NAME, "lineth-contract-addresses");
  assert.deepEqual(restored, value);
  assert.equal(restored.expectedDeployments["l1-rollup.proxy"].expectedAddress.length, 42);
  assert.ok(data["addresses.json"]);
});

test("replaces ConfigMap data with a resource-version guard", () => {
  assert.deepEqual(configMapDataPatch("42", { "addresses.json": '{"schemaVersion":1}' }), [
    { op: "test", path: "/metadata/resourceVersion", value: "42" },
    { op: "add", path: "/data", value: { "addresses.json": '{"schemaVersion":1}' } },
  ]);
});

test("retries only transient Kubernetes write responses", () => {
  for (const status of [429, 500, 502, 503, 504]) {
    assert.equal(isRetryableKubernetesStatus(status), true);
  }
  for (const status of [400, 401, 403, 404, 409, 422]) {
    assert.equal(isRetryableKubernetesStatus(status), false);
  }
  assert.equal(isKubernetesConflictStatus(409), true);
  assert.equal(isKubernetesConflictStatus(422), true);
});

test("recognizes only an exact checkpoint write after an ambiguous response", () => {
  const expected = { "checkpoint.json": '{"version":2}', "addresses.json": '{"version":2}' };

  assert.equal(configMapContainsData({ data: expected }, expected), true);
  assert.equal(
    configMapContainsData(
      { data: { "checkpoint.json": '{"version":1}', "addresses.json": '{"version":1}' } },
      expected,
    ),
    false,
  );
});

test("hashes normalized public deployment settings", () => {
  const first = deploymentConfigurationHash({
    rateLimitPeriod: "086400",
    rateLimitAmount: "01000",
    roles: {
      l1SecurityCouncil: L1_SIGNER.toLowerCase(),
      l1RollupOperators: [L1_SIGNER.toLowerCase()],
      l2SecurityCouncil: L2_SIGNER.toLowerCase(),
      l1L2MessageSetter: L2_SIGNER.toLowerCase(),
    },
  });
  const second = deploymentConfigurationHash({
    rateLimitPeriod: "86400",
    rateLimitAmount: "1000",
    roles: {
      l1SecurityCouncil: L1_SIGNER,
      l1RollupOperators: [L1_SIGNER],
      l2SecurityCouncil: L2_SIGNER,
      l1L2MessageSetter: L2_SIGNER,
    },
  });

  assert.equal(first, second);
});

test("rejects incomplete or malformed persisted deployment records", () => {
  const value = checkpoint();
  const deployment = value.expectedDeployments["l1-rollup.verifier"]!;
  value.deployments["l1-rollup.verifier"] = {
    address: deployment.expectedAddress,
    transactionHash: "not-a-transaction-hash",
    blockNumber: -1,
    chainId: "not-a-chain-id",
    recovered: false,
  };

  assert.throws(() => parseCheckpoint(JSON.stringify(value)), /invalid deployment record/);
});

test("rejects missing or unsafe persisted starting nonces", () => {
  const missing = checkpoint() as Partial<ReturnType<typeof checkpoint>>;
  delete missing.startingNonces;
  assert.throws(() => parseCheckpoint(JSON.stringify(missing)), /starting nonces/);

  const unsafe = checkpoint();
  unsafe.startingNonces.l1 = Number.MAX_SAFE_INTEGER + 1;
  assert.throws(() => parseCheckpoint(JSON.stringify(unsafe)), /starting nonces/);
});

test("rejects checkpoints missing artifact or in-flight state", () => {
  const missingArtifact = checkpoint() as Partial<ReturnType<typeof checkpoint>>;
  delete missingArtifact.artifactDigest;
  assert.throws(() => parseCheckpoint(JSON.stringify(missingArtifact)), /missing required fields/);

  const missingInFlight = checkpoint() as Partial<ReturnType<typeof checkpoint>>;
  delete missingInFlight.inFlightDeployments;
  assert.throws(() => parseCheckpoint(JSON.stringify(missingInFlight)), /missing required fields/);
});

test("rejects a malformed persisted artifact digest", () => {
  const malformed = checkpoint();
  malformed.artifactDigest = "sha256:1234";
  assert.throws(() => parseCheckpoint(JSON.stringify(malformed)), /artifact digest/);

  const uppercase = checkpoint();
  uppercase.artifactDigest = `sha256:${"AB".repeat(32)}`;
  assert.throws(() => parseCheckpoint(JSON.stringify(uppercase)), /artifact digest/);
});

test("validates persisted in-flight deployments against the deterministic plan", () => {
  const key = "l1-rollup.verifier";
  const valid = checkpoint();
  valid.inFlightDeployments[key] = { ...valid.expectedDeployments[key]! };
  assert.deepEqual(parseCheckpoint(JSON.stringify(valid)).inFlightDeployments, valid.inFlightDeployments);

  const mutations: Array<[label: string, mutate: (value: ReturnType<typeof checkpoint>) => void]> = [
    ["unknown key", (value) => (value.inFlightDeployments.unknown = { ...value.expectedDeployments[key]! })],
    ["wrong chain", (value) => (value.inFlightDeployments[key]!.chain = "l2")],
    ["unsafe nonce", (value) => (value.inFlightDeployments[key]!.nonce = Number.MAX_SAFE_INTEGER + 1)],
    ["malformed address", (value) => (value.inFlightDeployments[key]!.expectedAddress = "0x1234")],
    ["empty contract name", (value) => (value.inFlightDeployments[key]!.contractName = "")],
  ];

  for (const [label, mutate] of mutations) {
    const value = checkpoint();
    value.inFlightDeployments[key] = { ...value.expectedDeployments[key]! };
    mutate(value);
    assert.throws(() => parseCheckpoint(JSON.stringify(value)), /in-flight deployment/, label);
  }
});

test("fails closed when a deployment broadcast may be unresolved", () => {
  const value = checkpoint();
  assert.doesNotThrow(() => assertNoInFlightDeployments(value));

  const key = "l1-rollup.verifier";
  value.inFlightDeployments[key] = { ...value.expectedDeployments[key]! };

  assert.throws(
    () => assertNoInFlightDeployments(value),
    new RegExp(
      `checkpoint contains unresolved in-flight deployment ${key} at nonce ${value.expectedDeployments[key]!.nonce}; ` +
        "transaction broadcast is ambiguous and requires operator reconciliation",
    ),
  );
});
