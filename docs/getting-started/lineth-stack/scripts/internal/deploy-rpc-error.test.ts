import * as assert from "node:assert/strict";

import { formatQuickstartDeployRpcError } from "./deploy-rpc-error";

type TestCase = {
  name: string;
  run: () => void;
};

const tests: TestCase[] = [
  {
    name: "turns ethers already-known sendRawTransaction errors into actionable RPC guidance",
    run: () => {
      const rawTx = `0x${"ab".repeat(2048)}`;
      const formatted = formatQuickstartDeployRpcError({
        code: "UNKNOWN_ERROR",
        message: "could not coalesce error",
        error: {
          code: -32000,
          message: "already known",
        },
        payload: {
          method: "eth_sendRawTransaction",
          params: [rawTx],
        },
      });

      assert.ok(formatted instanceof Error);
      assert.match(formatted.message, /already known/);
      assert.match(formatted.message, /free or overloaded Sepolia RPC/i);
      assert.match(formatted.message, /L1_RPC_URL/);
      assert.match(formatted.message, /\.\/scripts\/reset\.sh/);
      assert.doesNotMatch(formatted.message, new RegExp(rawTx));
    },
  },
  {
    name: "leaves unrelated deployment errors untouched",
    run: () => {
      const error = new Error("constructor reverted");

      assert.equal(formatQuickstartDeployRpcError(error), undefined);
    },
  },
];

for (const test of tests) {
  test.run();
  console.log(`ok - ${test.name}`);
}
