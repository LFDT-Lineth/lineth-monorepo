import { appendDeployTimingRecord } from "./lib/timing";

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
  appendDeployTimingRecord(outPath, {
    name,
    status,
    startedMs: parseMillis(startedRaw, "startedMs"),
    endedMs: parseMillis(endedRaw, "endedMs"),
  });
}

main();
