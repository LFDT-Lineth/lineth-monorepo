// Shared deploy-timing JSONL writer for the quickstart internal TypeScript.

import * as fs from "node:fs";
import * as path from "node:path";

export type DeployTimingStatus = "ok" | "failed" | "error" | string;

/**
 * Appends a single deploy-timing record as one JSON line. The field order and
 * the durationSeconds rounding are part of the on-disk JSONL contract shared by
 * deploy-timing.ts and fund-runtime-accounts.ts.
 */
export function appendDeployTimingRecord(
  filePath: string,
  record: { name: string; status: DeployTimingStatus; startedMs: number; endedMs: number },
): void {
  fs.mkdirSync(path.dirname(filePath), { recursive: true });
  const durationMs = Math.max(0, record.endedMs - record.startedMs);
  fs.appendFileSync(
    filePath,
    `${JSON.stringify({
      name: record.name,
      status: record.status,
      startedAt: new Date(record.startedMs).toISOString(),
      endedAt: new Date(record.endedMs).toISOString(),
      durationMs,
      durationSeconds: Number((durationMs / 1000).toFixed(3)),
    })}\n`,
  );
}
