import { parseJestLog } from "./parse-jest-log";

// Raw log fixture copied from the GitHub Actions E2E job below.
// Source: https://github.com/Consensys/linea-monorepo/actions/runs/23834802065/job/69476754215
const RAW_LOG = `pnpm run -F e2e test:local
  shell: /usr/bin/bash -e {0}
  env:
    BESU_PACKAGE_TAG: dev-20260401061901-67f147a
    DOCKERHUB_USERNAME: ***
    DOCKERHUB_TOKEN: ***
    PNPM_HOME: /home/runner/setup-pnpm/node_modules/.bin
    COORDINATOR_TAG: 67f147a
    POSTMAN_TAG: 67f147a
    TRANSACTION_EXCLUSION_API_TAG: 67f147a
    PROVER_TAG: 2fc4392

> e2e@1.0.0 test:local /home/runner/_work/linea-monorepo/linea-monorepo/e2e
> pnpm run test:local:run --testPathIgnorePatterns=linea-besu-fleet.spec.ts --testPathIgnorePatterns=liveness.spec.ts && pnpm run test:liveness:local


> e2e@1.0.0 test:local:run /home/runner/_work/linea-monorepo/linea-monorepo/e2e
> TEST_ENV=local npx jest --testPathIgnorePatterns=linea-besu-fleet.spec.ts --testPathIgnorePatterns=liveness.spec.ts

timestamp=2026-04-01T06:24:19.543Z level=info | message=Configuring once-off prerequisite contracts
timestamp=2026-04-01T06:24:27.711Z level=info | message=L1 Dummy contract deployed. address=0x610178da211fef7d417bc0e6fed39f05609ad788
timestamp=2026-04-01T06:24:27.711Z level=info | message=L2 Dummy contract deployed. address=0xe4392c8ecc46b304c83cdb5edaf742899b1bda93
timestamp=2026-04-01T06:24:27.711Z level=info | message=L2 Test contract deployed. address=0x997fc3af1f193cbdc013060076c67a13e218980e
timestamp=2026-04-01T06:24:27.711Z level=info | message=L2 Poseidon2 contract deployed. address=0xfcc2155b495b6bf6701eb322d3a97b7817898306
timestamp=2026-04-01T06:24:27.711Z level=info | message=L2 LineaSequencerUptimeFeed contract deployed. address=0x7917abb0cdbf3d3c4057d6a2808ee85ec16260c1
timestamp=2026-04-01T06:24:27.711Z level=info | message=L2 SparseMerkleProof contract deployed. address=0x670365526a9971e4a225c38538c5d7ac248e4087
timestamp=2026-04-01T06:24:27.711Z level=info | message=LineaRollup funded with 500 ETH on L1
timestamp=2026-04-01T06:24:27.711Z level=info | message=Generating L2 traffic...
timestamp=2026-04-01T06:24:39.797Z level=info | message=L2 traffic generation started.
PASS src/common/test-helpers/deny-list.spec.ts
  deny-list helper
    ✓ should append lowercase addresses (6 ms)
    ✓ should insert a separator newline when existing file has no trailing newline
    ✓ should remove only target addresses case-insensitively (1 ms)
    ✓ should reload before and after callback and restore deny-list content (2 ms)
    ✓ should clean up and reload even when callback throws (4 ms)

PASS src/transaction-exclusion.spec.ts (8.154 s)
  Transaction exclusion test suite
    ✓ Should get the status of the rejected transaction reported from Besu RPC node (7545 ms)
    ○ skipped Should get the status of the rejected transaction reported from Besu SEQUENCER node

PASS src/l2.spec.ts (8.867 s)
  Layer 2 test suite
    ✓ Should revert if transaction data size is above the limit (4166 ms)
    ✓ Should succeed if transaction data size is below the limit (8212 ms)
    ✓ Should successfully send a legacy transaction (8196 ms)
    ✓ Should successfully send an EIP1559 transaction (8185 ms)
    ✓ Should successfully send an access list transaction with empty access list (8195 ms)
    ✓ Should successfully send an access list transaction with access list (8198 ms)
    ○ skipped Shomei frontend always behind while conflating multiple blocks and proving on L1

PASS src/opcodes.spec.ts (9.05 s)
  Opcodes test suite
    ✓ Should be able to execute all opcodes (8423 ms)
    ✓ Should be able to execute precompiles (P256VERIFY, KZG) (8194 ms)
    ✓ Should be able to execute G1 BLS precompiles (8199 ms)
    ✓ Should be able to execute G2 BLS precompiles (8206 ms)
    ✓ Should be able to execute BLS pairing and map precompiles (8217 ms)

PASS src/eip7702.spec.ts (20.895 s)
  EIP-7702 test suite
    ✓ should execute EIP-7702 (Set Code) transactions (16238 ms)
    ✓ should execute EIP-7702 self-call with no calldata when delegating to a codeless EOA (8170 ms)
    ✓ should block EIP-7702 tx when authorization_list authority is denylisted (16260 ms)
    ✓ should block EIP-7702 tx when authorization_list delegates to denylisted contract (12241 ms)
    ✓ should block EIP-7702 tx after non-denied authority is added to denylist and plugin config is reloaded (20274 ms)

PASS src/send-bundle.spec.ts (44.872 s)
  Send bundle test suite
    ✓ Call sendBundle to RPC node and the bundled txs should get included (16223 ms)
    ✓ Call sendBundle to RPC node but the bundled txs should not get included as not all of them are valid (16224 ms)
    ✓ Call sendBundle to RPC node and then cancelBundle to sequencer and no bundled txs should get included (44294 ms)

PASS src/submission-finalization.spec.ts (60.996 s)
  Submission and finalization test suite
    Contracts v6
      ✓ Check L1 data submission and finalization (25151 ms)
      ✓ Check L2 safe/finalized tag update on sequencer (60368 ms)

PASS src/shomei-get-proof.spec.ts (61.014 s)
  Shomei Linea get proof test suite
    ✓ Call linea_getProof to Shomei frontend node and get a valid proof (60435 ms)

timestamp=2026-04-01T06:25:44.004Z level=info test=Coordinator restart test suite When the coordinator restarts it should resume anchoring | message=Successfully anchored L1 -> L2 message before coordinator restart.
PASS src/messaging.spec.ts (75.043 s)
  Messaging test suite
    ✓ Should send a transaction with fee and calldata to L1 message service, be successfully claimed it on L2 (74410 ms)
    ✓ Should send a transaction with fee and without calldata to L1 message service, be successfully claimed it on L2 (70399 ms)
    ✓ Should send a transaction without fee and without calldata to L1 message service, be successfully claimed it on L2 (72406 ms)
    ✓ Should send a transaction with fee and calldata to L2 message service, be successfully claimed it on L1 (40302 ms)
    ✓ Should send a transaction with fee and without calldata to L2 message service, be successfully claimed it on L1 (42307 ms)

PASS src/bridge-tokens.spec.ts (78.598 s)
  Bridge ERC20 Tokens L1 -> L2 and L2 -> L1
    ✓ Bridge a token from L1 to L2 (77956 ms)
    ✓ Bridge a token from L2 to L1 (43429 ms)

PASS src/restart.spec.ts (158.969 s)
  Coordinator restart test suite
    ✓ When the coordinator restarts it should resume blob submission and finalization (87061 ms)
    ✓ When the coordinator restarts it should resume anchoring (158315 ms)

Test Suites: 11 passed, 11 total
Tests:       2 skipped, 37 passed, 39 total
Snapshots:   0 total
Time:        159.28 s
Ran all test suites.

> e2e@1.0.0 test:liveness:local /home/runner/_work/linea-monorepo/linea-monorepo/e2e
> TEST_ENV=local npx jest liveness.spec.ts

timestamp=2026-04-01T06:27:21.734Z level=info | message=Generating L2 traffic...
timestamp=2026-04-01T06:27:25.784Z level=info | message=L2 traffic generation started.
PASS src/liveness.spec.ts (13.828 s)
  Liveness test suite
    ✓ Should succeed to send liveness transactions after sequencer restarted (13354 ms)

Test Suites: 1 passed, 1 total
Tests:       1 passed, 1 total
Snapshots:   0 total
Time:        13.852 s
Ran all test suites matching liveness.spec.ts.`;

const FAILED_RAW_LOG = `FAIL src/l2.spec.ts (8.867 s)
  Layer 2 test suite
    ✓ Should revert if transaction data size is above the limit (4166 ms)

Test Suites: 1 failed, 1 total
Tests:       1 failed, 1 total
Snapshots:   0 total
Time:        8.88 s
Ran all test suites.`;

const INFRA_FAILURE_RAW_LOG = `Run nick-fields/retry@ec71cc2ab81d554ebbe88c79ab5975992d79ba08
Attempt 1
Error: Timeout of 600000ms hit
List docker containers/images`;

const EXPLICIT_TIMEOUT_FAILURE_RAW_LOG = `FAIL src/shomei-get-proof.spec.ts (150.801 s)
  Shomei Linea get proof test suite
    ✕ Call linea_getProof to Shomei frontend node and get a valid proof (150004 ms)

  ● Shomei Linea get proof test suite › Call linea_getProof to Shomei frontend node and get a valid proof

    thrown: "Exceeded timeout of 150000 ms for a test.
    Add a timeout value to this test to increase the timeout, if this is a long-running test. See https://jestjs.io/docs/api#testname-fn-timeout."

Test Suites: 1 failed, 1 total
Tests:       1 failed, 1 total
Snapshots:   0 total
Time:        164.144 s
Ran all test suites.`;

describe("parseJestLog", () => {
  afterEach(() => {
    jest.restoreAllMocks();
  });

  it("parses all top-level spec files from the successful CI job log", () => {
    // Arrange
    const result = parseJestLog(RAW_LOG, "success");

    // Assert
    expect(result).toHaveLength(11);
    expect(result.map((spec) => spec.specFile)).toEqual(
      expect.arrayContaining([
        "src/transaction-exclusion.spec.ts",
        "src/l2.spec.ts",
        "src/opcodes.spec.ts",
        "src/eip7702.spec.ts",
        "src/send-bundle.spec.ts",
        "src/submission-finalization.spec.ts",
        "src/shomei-get-proof.spec.ts",
        "src/messaging.spec.ts",
        "src/bridge-tokens.spec.ts",
        "src/restart.spec.ts",
        "src/liveness.spec.ts",
      ]),
    );
  });

  it("captures skipped tests under the matching top-level spec", () => {
    // Arrange
    const result = parseJestLog(RAW_LOG, "success");

    // Act
    const transactionExclusionSpec = result.find((spec) => spec.specFile === "src/transaction-exclusion.spec.ts");
    const layer2Spec = result.find((spec) => spec.specFile === "src/l2.spec.ts");

    // Assert
    expect(transactionExclusionSpec).toEqual({
      specFile: "src/transaction-exclusion.spec.ts",
      status: "PASS",
      durationSeconds: 8.154,
      tests: [
        {
          name: "Should get the status of the rejected transaction reported from Besu RPC node",
          durationMs: 7545,
          status: "passed",
        },
        {
          name: "Should get the status of the rejected transaction reported from Besu SEQUENCER node",
          durationMs: 0,
          status: "skipped",
        },
      ],
    });
    expect(layer2Spec?.tests.at(-1)).toEqual({
      name: "Shomei frontend always behind while conflating multiple blocks and proving on L1",
      durationMs: 0,
      status: "skipped",
    });
  });

  it("excludes nested helper specs from the parsed result", () => {
    // Arrange
    const result = parseJestLog(RAW_LOG, "success");

    // Act
    const hasNestedHelperSpec = result.some((spec) => spec.specFile === "src/common/test-helpers/deny-list.spec.ts");

    // Assert
    expect(hasNestedHelperSpec).toBe(false);
  });

  it("does not leak nested helper spec tests into the preceding top-level spec when ordering is reversed", () => {
    // Arrange - nested helper spec appears AFTER a top-level spec (reversed from the typical fast-first ordering)
    const logWithReversedOrder = `PASS src/l2.spec.ts (8.867 s)
  Layer 2 test suite
    ✓ Should succeed if transaction data size is below the limit (8212 ms)

PASS src/common/test-helpers/deny-list.spec.ts
  deny-list helper
    ✓ should append lowercase addresses (6 ms)
    ✓ should remove only target addresses case-insensitively (1 ms)

Test Suites: 2 passed, 2 total
Tests:       3 passed, 3 total
Snapshots:   0 total
Time:        9 s
Ran all test suites.`;

    // Act
    const result = parseJestLog(logWithReversedOrder, "success");

    // Assert
    expect(result).toHaveLength(1);
    expect(result[0].specFile).toBe("src/l2.spec.ts");
    expect(result[0].tests).toHaveLength(1);
    expect(result[0].tests[0].name).toBe("Should succeed if transaction data size is below the limit");
  });

  it("parses the separate liveness command output as a top-level spec", () => {
    // Arrange
    const result = parseJestLog(RAW_LOG, "success");

    // Act
    const livenessSpec = result.at(-1);

    // Assert
    expect(livenessSpec).toEqual({
      specFile: "src/liveness.spec.ts",
      status: "PASS",
      durationSeconds: 13.828,
      tests: [
        {
          name: "Should succeed to send liveness transactions after sequencer restarted",
          durationMs: 13354,
          status: "passed",
        },
      ],
    });
  });

  it("does not synthesize TIMEOUT results when a failure happens before any top-level spec starts", () => {
    // Arrange
    const timeoutEligibleSpecFiles = ["src/l2.spec.ts", "src/messaging.spec.ts"];

    // Act
    const result = parseJestLog(INFRA_FAILURE_RAW_LOG, "failure", timeoutEligibleSpecFiles);

    // Assert
    expect(result).toEqual([]);
  });

  it("keeps explicit Jest timeout failures as FAIL results", () => {
    // Arrange
    const timeoutEligibleSpecFiles = ["src/shomei-get-proof.spec.ts", "src/messaging.spec.ts"];

    // Act
    const result = parseJestLog(EXPLICIT_TIMEOUT_FAILURE_RAW_LOG, "failure", timeoutEligibleSpecFiles);

    // Assert
    expect(result).toEqual([
      {
        specFile: "src/shomei-get-proof.spec.ts",
        status: "FAIL",
        durationSeconds: 150.801,
        tests: [
          {
            name: "Call linea_getProof to Shomei frontend node and get a valid proof",
            durationMs: 150004,
            status: "failed",
          },
        ],
      },
      {
        specFile: "src/messaging.spec.ts",
        status: "TIMEOUT",
        durationSeconds: NaN,
        tests: [],
      },
    ]);
  });

  it("adds TIMEOUT only for the provided timeout-eligible spec files on failure", () => {
    // Arrange
    const timeoutEligibleSpecFiles = ["src/l2.spec.ts", "src/messaging.spec.ts"];

    // Act
    const result = parseJestLog(FAILED_RAW_LOG, "failure", timeoutEligibleSpecFiles);

    // Assert
    expect(result).toEqual([
      {
        specFile: "src/l2.spec.ts",
        status: "FAIL",
        durationSeconds: 8.867,
        tests: [
          {
            name: "Should revert if transaction data size is above the limit",
            durationMs: 4166,
            status: "passed",
          },
        ],
      },
      {
        specFile: "src/messaging.spec.ts",
        status: "TIMEOUT",
        durationSeconds: NaN,
        tests: [],
      },
    ]);
  });
});
