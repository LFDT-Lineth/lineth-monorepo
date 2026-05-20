import * as fs from "fs";
import * as path from "path";

import { extractAnalysisEntriesFromLog, generateAnalysisReportHtml } from "./utils/analysisReport.js";

/**
 * Generates a human-readable HTML report from a `run.ts` log file.
 *
 * Typical usage:
 * 1) Capture monitor run output:
 *    pnpm --filter @consensys/lido-governance-monitor exec tsx run.ts > run.log
 * 2) Generate HTML report:
 *    pnpm --filter @consensys/lido-governance-monitor exec tsx scripts/generate-analysis-report.ts run.log
 * 3) Open the generated `run.analysis-report.html` file in your browser.
 */

const CLI_ERROR = {
  USAGE: "Usage: tsx scripts/generate-analysis-report.ts <log-file-path> [output-html-path]",
  INPUT_NOT_FOUND: "Input log file not found:",
} as const;

export function resolveOutputPath(inputPath: string, explicitOutputPath?: string): string {
  if (explicitOutputPath) {
    return path.resolve(explicitOutputPath);
  }
  const parsedPath = path.parse(inputPath);
  return path.resolve(parsedPath.dir, `${parsedPath.name}.analysis-report.html`);
}

export function runCli(argv: string[] = process.argv): number {
  const [, , inputPathArg, outputPathArg] = argv;
  if (!inputPathArg) {
    console.error(CLI_ERROR.USAGE);
    return 1;
  }

  const inputPath = path.resolve(inputPathArg);
  if (!fs.existsSync(inputPath)) {
    console.error(`${CLI_ERROR.INPUT_NOT_FOUND} ${inputPath}`);
    return 1;
  }

  const outputPath = resolveOutputPath(inputPath, outputPathArg);
  const logText = fs.readFileSync(inputPath, "utf-8");
  const entries = extractAnalysisEntriesFromLog(logText);
  const html = generateAnalysisReportHtml(entries, path.basename(inputPath));
  fs.writeFileSync(outputPath, html, "utf-8");

  console.log(`Generated analysis report: ${outputPath}`);
  console.log(`Total analyses: ${entries.length}`);
  return 0;
}

const EXECUTABLE_NAME = "generate-analysis-report";
const executedPath = process.argv[1] ? path.basename(process.argv[1]) : "";
if (executedPath.includes(EXECUTABLE_NAME)) {
  process.exit(runCli(process.argv));
}
