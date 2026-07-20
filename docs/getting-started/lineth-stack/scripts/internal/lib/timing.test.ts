import * as assert from "node:assert/strict";
import * as fs from "node:fs";
import * as os from "node:os";
import * as path from "node:path";

import { appendDeployTimingRecord } from "./timing";

type TestCase = {
  name: string;
  run: () => Promise<void> | void;
};

function tmpDir(): string {
  return fs.mkdtempSync(path.join(os.tmpdir(), "lineth-lib-timing-"));
}

const tests: TestCase[] = [
  {
    name: "appends one JSON line with expected fields, ISO timestamps, and rounding",
    run: () => {
      const dir = tmpDir();
      const file = path.join(dir, "nested", "deploy-timing.jsonl");
      appendDeployTimingRecord(file, { name: "step-1", status: "ok", startedMs: 1_000, endedMs: 2_500 });

      const lines = fs.readFileSync(file, "utf8").trimEnd().split("\n");
      assert.equal(lines.length, 1);
      const record = JSON.parse(lines[0]);
      assert.deepEqual(Object.keys(record), [
        "name",
        "status",
        "startedAt",
        "endedAt",
        "durationMs",
        "durationSeconds",
      ]);
      assert.equal(record.name, "step-1");
      assert.equal(record.status, "ok");
      assert.equal(record.startedAt, new Date(1_000).toISOString());
      assert.equal(record.endedAt, new Date(2_500).toISOString());
      assert.equal(record.durationMs, 1_500);
      assert.equal(record.durationSeconds, 1.5);
    },
  },
  {
    name: "rounds durationSeconds to three decimals and clamps negative durations",
    run: () => {
      const dir = tmpDir();
      const file = path.join(dir, "deploy-timing.jsonl");
      appendDeployTimingRecord(file, { name: "round", status: "ok", startedMs: 0, endedMs: 1_2345 });
      appendDeployTimingRecord(file, { name: "negative", status: "failed", startedMs: 5_000, endedMs: 1_000 });

      const lines = fs.readFileSync(file, "utf8").trimEnd().split("\n");
      assert.equal(lines.length, 2);
      assert.equal(JSON.parse(lines[0]).durationSeconds, 12.345);
      assert.equal(JSON.parse(lines[1]).durationMs, 0);
      assert.equal(JSON.parse(lines[1]).durationSeconds, 0);
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
