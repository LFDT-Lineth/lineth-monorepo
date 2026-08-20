import assert from "node:assert/strict";
import test from "node:test";

import { formatDeploymentRecord, parseDeploymentRecord } from "../../common/helpers/deploymentRecord";

const ADDRESS = "0x1000000000000000000000000000000000000001";
const TX_HASH = `0x${"ab".repeat(32)}`;

test("formats and parses a durable deployment record", () => {
  const line = formatDeploymentRecord({
    contractName: "LinethRollupV8",
    address: ADDRESS,
    transactionHash: TX_HASH,
    blockNumber: 42,
    chainId: 1337n,
  });

  assert.equal(
    line,
    `contract=LinethRollupV8 deployed: address=${ADDRESS} blockNumber=42 chainId=1337 txHash=${TX_HASH}`,
  );
  assert.deepEqual(parseDeploymentRecord(line), {
    contractName: "LinethRollupV8",
    address: ADDRESS,
    transactionHash: TX_HASH,
    blockNumber: 42,
    chainId: "1337",
  });
});

test("ignores ordinary logs and legacy records without transaction hashes", () => {
  assert.equal(parseDeploymentRecord("Deploying LinethRollupV8..."), undefined);
  assert.equal(
    parseDeploymentRecord(`contract=LinethRollupV8 deployed: address=${ADDRESS} blockNumber=42 chainId=1337`),
    undefined,
  );
});
