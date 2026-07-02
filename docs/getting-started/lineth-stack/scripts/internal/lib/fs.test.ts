import * as assert from "node:assert/strict";
import * as fs from "node:fs";
import * as os from "node:os";
import * as path from "node:path";

import { ensureDir, writeFileAtomic } from "./fs";

type TestCase = {
  name: string;
  run: () => Promise<void> | void;
};

function tmpDir(): string {
  return fs.mkdtempSync(path.join(os.tmpdir(), "lineth-lib-fs-"));
}

const tests: TestCase[] = [
  {
    name: "writeFileAtomic creates the file with the requested mode and no leftover temp file",
    run: () => {
      const dir = tmpDir();
      const file = path.join(dir, "secret.txt");
      writeFileAtomic(file, "data\n", 0o600);

      assert.equal(fs.readFileSync(file, "utf8"), "data\n");
      assert.equal(fs.statSync(file).mode & 0o777, 0o600);
      assert.equal(fs.existsSync(`${file}.tmp`), false);
    },
  },
  {
    name: "writeFileAtomic preserves the 0o644 container-readable mode and overwrites existing content",
    run: () => {
      const dir = tmpDir();
      const file = path.join(dir, "addresses.json");
      writeFileAtomic(file, "first\n", 0o644);
      writeFileAtomic(file, "second\n", 0o644);

      assert.equal(fs.readFileSync(file, "utf8"), "second\n");
      assert.equal(fs.statSync(file).mode & 0o777, 0o644);
      assert.equal(fs.existsSync(`${file}.tmp`), false);
    },
  },
  {
    name: "writeFileAtomic defaults to 0o600 when no mode is provided",
    run: () => {
      const dir = tmpDir();
      const file = path.join(dir, "default-mode.txt");
      writeFileAtomic(file, "x\n");

      assert.equal(fs.statSync(file).mode & 0o777, 0o600);
    },
  },
  {
    name: "ensureDir is idempotent and creates nested directories",
    run: () => {
      const dir = tmpDir();
      const nested = path.join(dir, "a", "b", "c");
      ensureDir(nested);
      ensureDir(nested);

      assert.equal(fs.existsSync(nested), true);
      assert.equal(fs.statSync(nested).isDirectory(), true);
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
