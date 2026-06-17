import * as fs from "node:fs";
import * as path from "node:path";

const DEFAULT_PATH = "/deployments/deploy-timing.jsonl";

function usage(): never {
  throw new Error("usage: deploy-timing.ts record <path> <name> <startedMs> <endedMs> <status>");
}

function parseMillis(value: string, label: string): number {
  if (!/^[0-9]+$/.test(value)) {
    throw new Error(`${label} must be epoch milliseconds`);
  }
  return Number(value);
}

function main() {
  const [command, maybePath, name, startedRaw, endedRaw, status] = process.argv.slice(2);
  if (command !== "record" || !name || !startedRaw || !endedRaw || !status) {
    usage();
  }

  const outPath = maybePath || process.env.DEPLOY_TIMING_PATH || DEFAULT_PATH;
  const startedMs = parseMillis(startedRaw, "startedMs");
  const endedMs = parseMillis(endedRaw, "endedMs");
  const durationMs = Math.max(0, endedMs - startedMs);

  fs.mkdirSync(path.dirname(outPath), { recursive: true });
  fs.appendFileSync(
    outPath,
    `${JSON.stringify({
      name,
      status,
      startedAt: new Date(startedMs).toISOString(),
      endedAt: new Date(endedMs).toISOString(),
      durationMs,
      durationSeconds: Number((durationMs / 1000).toFixed(3)),
    })}\n`,
  );
}

main();
