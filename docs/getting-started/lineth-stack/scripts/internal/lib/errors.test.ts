import * as assert from "node:assert/strict";

import { sanitizeExternalError, sanitizeSecrets } from "./errors";

type TestCase = {
  name: string;
  run: () => Promise<void> | void;
};

const tests: TestCase[] = [
  {
    name: "sanitizeExternalError redacts URLs and 32-byte hex",
    run: () => {
      const message = sanitizeExternalError(
        new Error("failed calling https://secret.example/rpc?token=abc with 0x" + "a".repeat(64)),
      );
      assert.match(message, /<redacted-url>/);
      assert.match(message, /<redacted-hex>/);
      assert.doesNotMatch(message, /secret\.example/);
      assert.doesNotMatch(message, /0x[a-fA-F0-9]{64}/);
    },
  },
  {
    name: "sanitizeExternalError stringifies non-Error values",
    run: () => {
      assert.equal(sanitizeExternalError("plain text"), "plain text");
      assert.equal(sanitizeExternalError(42), "42");
    },
  },
  {
    name: "sanitizeSecrets removes provided secrets and case variants",
    run: () => {
      const secret = "SuperSecretValue";
      const message = sanitizeSecrets(
        new Error(`leaked ${secret} and ${secret.toLowerCase()} and ${secret.toUpperCase()}`),
        [secret],
      );
      assert.match(message, /leaked \[REDACTED\] and \[REDACTED\] and \[REDACTED\]/);
      assert.equal(message.includes(secret), false);
    },
  },
  {
    name: "sanitizeSecrets ignores empty/undefined secrets and defaults to none",
    run: () => {
      assert.equal(sanitizeSecrets(new Error("nothing to redact")), "nothing to redact");
      assert.equal(sanitizeSecrets(new Error("keep this"), ["", undefined]), "keep this");
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
