// Shared filesystem helpers for the quickstart internal TypeScript.

import * as fs from "node:fs";

export function ensureDir(dir: string): void {
  fs.mkdirSync(dir, { recursive: true });
}

/**
 * Atomic write: write to a temp file in the same directory, rename it into
 * place, then chmod to `mode`.
 *
 * Callers MUST pass the intended mode explicitly when it is not the default
 * 0o600; the quickstart deliberately keeps secret material at 0o600 while
 * container-readable dev artifacts use 0o644. Do not unify these modes.
 */
export function writeFileAtomic(file: string, contents: string, mode = 0o600): void {
  const tmp = `${file}.tmp`;
  fs.writeFileSync(tmp, contents, { mode });
  fs.renameSync(tmp, file);
  fs.chmodSync(file, mode);
}
