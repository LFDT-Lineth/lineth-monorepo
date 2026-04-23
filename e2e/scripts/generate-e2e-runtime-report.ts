/*
  *******************************************************************************************
  Fetches completed E2E workflow runs from the last N days and generates an HTML report
  showing per-spec pass/fail/timeout status and runtime trends over time. Also writes a
  JSON summary used by the CI workflow to post a digest to Slack.

  Qualifiers:
  - Only scrapes E2E workflow runs from push to `main`.
  - Only gets results from the latest retry for each individual job.

  Run by: .github/workflows/slack-notify-e2e-runtime-report.yml

  -------------------------------------------------------------------------------------------
  Example (local):
  -------------------------------------------------------------------------------------------
  GITHUB_TOKEN=<token> \
  pnpm -F e2e run generate-runtime-report -- --days 30
  *******************************************************************************************
*/
import { Octokit } from "@octokit/rest";
import { writeFileSync } from "node:fs";

import { computeFlakyTests, type FlakyTestEntry } from "./helpers/compute-flaky-tests";
import { discoverTimeoutEligibleSpecFiles } from "./helpers/discover-timeout-eligible-spec-files";
import { parseJestLog, type JobConclusion, type SpecResult, type SpecStatus } from "./helpers/parse-jest-log";

interface E2eRun {
  runId: number;
  runAttempt: number;
  jobId: number;
  jobConclusion: JobConclusion;
  startedAt: string;
  commitSha: string;
  durationSeconds: number;
}

interface E2eRunResult {
  run: E2eRun;
  specResults: SpecResult[];
}

interface ReportSummary {
  totalRuns: number;
  passedRuns: number;
  failedRuns: number;
  dateRange: string;
  medianRunDurationSeconds: number;
  maxRunDurationSeconds: number;
  topFlakyTests: FlakyTestEntry[];
  flakySlackText: string;
}

const DEFAULT_DAYS = 30;
const OWNER = "Consensys";
const REPO = "linea-monorepo";
const WORKFLOW_FILE = "main.yml";
const E2E_JOB_NAME = "run-e2e-tests / run-e2e-tests";

function parseArgs(): { days: number } {
  const args = process.argv.slice(2);
  const daysIndex = args.indexOf("--days");
  const days = daysIndex !== -1 && args[daysIndex + 1] ? parseInt(args[daysIndex + 1], 10) : DEFAULT_DAYS;
  if (isNaN(days) || days < 1 || days > 365) {
    console.error("Invalid --days value. Must be between 1 and 365.");
    process.exit(1);
  }
  return { days };
}

async function fetchE2eRuns(octokit: Octokit, days: number): Promise<{ run: E2eRun; rawLog: string }[]> {
  const since = new Date();
  // Subtract an extra day because GitHub's created:>DATE filter treats the date as a day
  // boundary (exclusive), so >2026-03-27 means "Mar 28 or later" and drops all of Mar 27.
  since.setDate(since.getDate() - days - 1);
  const sinceStr = since.toISOString().split("T")[0];

  console.log(`Fetching workflow runs since ${sinceStr}...`);

  const runs = await octokit.paginate(octokit.actions.listWorkflowRuns, {
    owner: OWNER,
    repo: REPO,
    workflow_id: WORKFLOW_FILE,
    branch: "main",
    event: "push",
    status: "completed",
    created: `>${sinceStr}`,
    per_page: 100,
  });

  console.log(`Found ${runs.length} completed workflow runs.`);

  const results: { run: E2eRun; rawLog: string }[] = [];

  for (const workflowRun of runs) {
    const { data: jobsData } = await octokit.actions.listJobsForWorkflowRun({
      owner: OWNER,
      repo: REPO,
      run_id: workflowRun.id,
      filter: "latest",
      per_page: 100,
    });

    const e2eJob = jobsData.jobs.find(
      (job) => job.name === E2E_JOB_NAME && (job.conclusion === "success" || job.conclusion === "failure"),
    );

    if (!e2eJob) continue;

    let rawLog: string;
    try {
      const { data } = await octokit.actions.downloadJobLogsForWorkflowRun({
        owner: OWNER,
        repo: REPO,
        job_id: e2eJob.id,
      });
      rawLog = data as unknown as string;
    } catch {
      console.warn(`Warning: could not download logs for job ${e2eJob.id} (run ${workflowRun.id}), skipping.`);
      continue;
    }

    if (!rawLog) {
      console.warn(`Warning: empty log for job ${e2eJob.id} (run ${workflowRun.id}), skipping.`);
      continue;
    }

    const startedAt = e2eJob.started_at ?? workflowRun.created_at;
    const durationSeconds =
      e2eJob.completed_at && e2eJob.started_at
        ? (new Date(e2eJob.completed_at).getTime() - new Date(e2eJob.started_at).getTime()) / 1000
        : NaN;

    results.push({
      run: {
        runId: workflowRun.id,
        runAttempt: workflowRun.run_attempt ?? 1,
        jobId: e2eJob.id,
        jobConclusion: e2eJob.conclusion as JobConclusion,
        startedAt,
        commitSha: workflowRun.head_sha,
        durationSeconds,
      },
      rawLog,
    });

    console.log(`  Run ${workflowRun.id} (${e2eJob.conclusion}): fetched ${rawLog.length} bytes of logs`);
  }

  console.log(`Fetched logs for ${results.length} qualifying E2E runs.`);
  return results;
}

function median(values: number[]): number {
  if (values.length === 0) return NaN;
  const sorted = [...values].sort((a, b) => a - b);
  const mid = Math.floor(sorted.length / 2);
  return sorted.length % 2 !== 0 ? sorted[mid] : (sorted[mid - 1] + sorted[mid]) / 2;
}

function stddev(values: number[]): number {
  if (values.length < 2) return 0;
  const mean = values.reduce((a, b) => a + b, 0) / values.length;
  const variance = values.reduce((sum, v) => sum + (v - mean) ** 2, 0) / values.length;
  return Math.sqrt(variance);
}

type ColorLevel = "green" | "yellow" | "red" | "gray";

function getCellColor(duration: number, med: number, sd: number, status: SpecStatus): ColorLevel {
  if (status === "FAIL" || status === "TIMEOUT") return "red";
  if (isNaN(duration)) return "gray";
  if (sd === 0) return "green";
  if (duration > med + 2 * sd) return "red";
  if (duration > med + 1 * sd) return "yellow";
  return "green";
}

const COLOR_MAP: Record<ColorLevel, string> = {
  green: "#d4edda",
  yellow: "#fff3cd",
  red: "#f8d7da",
  gray: "#e2e3e5",
};

function renderHtmlReport(runResults: E2eRunResult[]): { html: string; summary: ReportSummary } {
  const allSpecFiles = [...new Set(runResults.flatMap((r) => r.specResults.map((s) => s.specFile)))].sort();

  // Compute per-spec stats from successful runs
  const specStats = new Map<
    string,
    { med: number; sd: number; min: number; max: number; count: number; failureCount: number }
  >();
  for (const specFile of allSpecFiles) {
    const durations = runResults
      .flatMap((r) => r.specResults)
      .filter((s) => s.specFile === specFile && s.status === "PASS" && !isNaN(s.durationSeconds))
      .map((s) => s.durationSeconds);

    const failureCount = runResults.filter((r) =>
      r.specResults.some((s) => s.specFile === specFile && (s.status === "FAIL" || s.status === "TIMEOUT")),
    ).length;

    specStats.set(specFile, {
      med: median(durations),
      sd: stddev(durations),
      min: durations.length > 0 ? Math.min(...durations) : NaN,
      max: durations.length > 0 ? Math.max(...durations) : NaN,
      count: durations.length,
      failureCount,
    });
  }

  // Sort spec files by median duration descending
  const sortedSpecs = [...allSpecFiles].sort((a, b) => {
    const medA = specStats.get(a)!.med;
    const medB = specStats.get(b)!.med;
    if (isNaN(medA) && isNaN(medB)) return 0;
    if (isNaN(medA)) return 1;
    if (isNaN(medB)) return -1;
    return medB - medA;
  });

  const totalRuns = runResults.length;
  const passedRuns = runResults.filter((r) => r.run.jobConclusion === "success").length;
  const failedRuns = totalRuns - passedRuns;

  const startDate = runResults[0].run.startedAt.split("T")[0];
  const endDate = runResults[runResults.length - 1].run.startedAt.split("T")[0];
  const dateRange = `${startDate} to ${endDate}`;

  const runDurations = runResults.map((r) => r.run.durationSeconds).filter((d) => !isNaN(d));
  const medianRunDuration = median(runDurations);
  const maxRunDuration = runDurations.length > 0 ? Math.max(...runDurations) : NaN;

  const allFlakyTests = computeFlakyTests(runResults);
  const topFlakyTests = allFlakyTests.slice(0, 3);
  const flakySlackText =
    topFlakyTests.length === 0
      ? "None detected this period."
      : topFlakyTests
          .map((t, i) => {
            const spec = t.specFile.replace("src/", "").replace(".spec.ts", "");
            return `${i + 1}. ${t.testName} (${spec}) - ${(t.failRate * 100).toFixed(1)}% fail rate (${t.failures}/${t.appearances})`;
          })
          .join("\\n");

  const summary: ReportSummary = {
    totalRuns,
    passedRuns,
    failedRuns,
    dateRange,
    medianRunDurationSeconds: Math.round(isNaN(medianRunDuration) ? 0 : medianRunDuration),
    maxRunDurationSeconds: Math.round(isNaN(maxRunDuration) ? 0 : maxRunDuration),
    topFlakyTests,
    flakySlackText,
  };

  // Build HTML
  const css = `
    body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; margin: 20px; color: #333; }
    h1, h2, h3 { color: #1a1a1a; }
    table { border-collapse: collapse; width: 100%; margin-bottom: 20px; }
    th, td { border: 1px solid #ddd; padding: 6px 10px; text-align: center; font-size: 13px; }
    th { background-color: #f5f5f5; font-weight: 600; position: sticky; top: 0; }
    td.spec-name { text-align: left; font-family: monospace; font-size: 12px; white-space: nowrap; }
    .header-stats { display: flex; gap: 24px; margin-bottom: 16px; }
    .stat { font-size: 14px; }
    .stat strong { font-size: 20px; }
    details { margin-bottom: 12px; }
    summary { cursor: pointer; font-weight: 600; font-family: monospace; padding: 4px 0; }
    .detail-table td { font-size: 12px; }
    .skipped { color: #888; font-style: italic; }
    .timeline-container { overflow-x: auto; }
  `;

  const MONTH_ABBR = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];

  // Timeline table header
  const timelineHeaders = runResults
    .map((r) => {
      const [, mm, dd] = r.run.startedAt.split("T")[0].split("-");
      const date = `${MONTH_ABBR[parseInt(mm, 10) - 1]} ${parseInt(dd, 10)}`;
      const sha = r.run.commitSha.slice(0, 7);
      const icon = r.run.jobConclusion === "failure" ? " X" : "";
      const jobUrl = `https://github.com/${OWNER}/${REPO}/actions/runs/${r.run.runId}/job/${r.run.jobId}`;
      return `<th title="${r.run.startedAt}\n${r.run.commitSha}"><a href="${jobUrl}" target="_blank" style="color:inherit;text-decoration:none;">${date}<br><span style="color:#0366d6;text-decoration:underline;">${sha}${icon}</span></a></th>`;
    })
    .join("\n            ");

  // Total run duration row
  const totalDurations = runResults.map((r) => r.run.durationSeconds);
  const totalDurationMedian = median(totalDurations.filter((d) => !isNaN(d)));
  const totalDurationSd = stddev(totalDurations.filter((d) => !isNaN(d)));
  const totalRunCells = runResults
    .map((r) => {
      if (isNaN(r.run.durationSeconds)) return `<td style="background:${COLOR_MAP.gray}">-</td>`;
      const color = getCellColor(
        r.run.durationSeconds,
        totalDurationMedian,
        totalDurationSd,
        r.run.jobConclusion === "failure" ? "FAIL" : "PASS",
      );
      return `<td style="background:${COLOR_MAP[color]};font-weight:600;">${(r.run.durationSeconds / 60).toFixed(1)}m</td>`;
    })
    .join("");
  const totalRunRow = `<tr style="border-bottom:2px solid #aaa;"><td class="spec-name" style="font-weight:600;">total run</td>${totalRunCells}</tr>`;

  // Timeline table rows
  const timelineRows = sortedSpecs
    .map((specFile) => {
      const stats = specStats.get(specFile)!;
      const cells = runResults
        .map((r) => {
          const spec = r.specResults.find((s) => s.specFile === specFile);
          if (!spec) return `<td style="background:${COLOR_MAP.gray}">-</td>`;

          const color = getCellColor(spec.durationSeconds, stats.med, stats.sd, spec.status);
          if (spec.status === "TIMEOUT") return `<td style="background:${COLOR_MAP.red}">TIMEOUT</td>`;
          if (spec.status === "FAIL")
            return `<td style="background:${COLOR_MAP.red}">FAIL ${spec.durationSeconds.toFixed(1)}s</td>`;
          return `<td style="background:${COLOR_MAP[color]}">${spec.durationSeconds.toFixed(1)}s</td>`;
        })
        .join("");

      const shortName = specFile.replace("src/", "").replace(".spec.ts", "");
      return `<tr><td class="spec-name">${shortName}</td>${cells}</tr>`;
    })
    .join("\n          ");

  // Total run summary row
  const totalRunDurations = runResults.map((r) => r.run.durationSeconds).filter((d) => !isNaN(d));
  const totalRunMin = totalRunDurations.length > 0 ? Math.min(...totalRunDurations) : NaN;
  const totalRunMax = totalRunDurations.length > 0 ? Math.max(...totalRunDurations) : NaN;
  const totalRunFailRate = totalRuns > 0 ? ((failedRuns / totalRuns) * 100).toFixed(1) : "0.0";
  const totalRunSummaryRow = `<tr style="border-bottom:2px solid #aaa;">
          <td class="spec-name" style="font-weight:600;">total run</td>
          <td style="background:${COLOR_MAP.green};font-weight:600;">${isNaN(medianRunDuration) ? "-" : (medianRunDuration / 60).toFixed(1) + "m"}</td>
          <td>${isNaN(totalRunMin) ? "-" : (totalRunMin / 60).toFixed(1) + "m"}</td>
          <td>${isNaN(totalRunMax) ? "-" : (totalRunMax / 60).toFixed(1) + "m"}</td>
          <td>${totalRunDurations.length}</td>
          <td>${failedRuns}</td>
          <td>${totalRunFailRate}%</td>
        </tr>`;

  // Summary table rows (color-code median cell: red if any run had FAIL/TIMEOUT)
  const summaryRows = sortedSpecs
    .map((specFile) => {
      const stats = specStats.get(specFile)!;
      const shortName = specFile.replace("src/", "").replace(".spec.ts", "");
      const medColor: ColorLevel = isNaN(stats.med) ? "gray" : stats.failureCount > 0 ? "red" : "green";
      const total = stats.count + stats.failureCount;
      const failRate = total > 0 ? ((stats.failureCount / total) * 100).toFixed(1) : "0.0";
      return `<tr>
            <td class="spec-name">${shortName}</td>
            <td style="background:${COLOR_MAP[medColor]}">${isNaN(stats.med) ? "-" : stats.med.toFixed(1)}</td>
            <td>${isNaN(stats.min) ? "-" : stats.min.toFixed(1)}</td>
            <td>${isNaN(stats.max) ? "-" : stats.max.toFixed(1)}</td>
            <td>${stats.count}</td>
            <td>${stats.failureCount}</td>
            <td>${failRate}%</td>
          </tr>`;
    })
    .join("\n          ");

  const LEADERBOARD_VISIBLE_ROWS = 10;
  const FLAKY_RED_THRESHOLD = 0.3;
  const FLAKY_YELLOW_THRESHOLD = 0.1;

  let flakySection: string;
  if (allFlakyTests.length === 0) {
    flakySection = `<p>No flaky tests detected in this period.</p>`;
  } else {
    const flakyRowHtml = (entry: FlakyTestEntry, rank: number) => {
      const bg =
        entry.failRate >= FLAKY_RED_THRESHOLD
          ? `background:${COLOR_MAP.red}`
          : entry.failRate >= FLAKY_YELLOW_THRESHOLD
            ? `background:${COLOR_MAP.yellow}`
            : "";
      const shortSpec = entry.specFile.replace("src/", "").replace(".spec.ts", "");
      return `<tr style="${bg}">
              <td>${rank}</td>
              <td style="text-align:left">${entry.testName}</td>
              <td class="spec-name">${shortSpec}</td>
              <td>${entry.failures}</td>
              <td>${entry.appearances}</td>
              <td>${(entry.failRate * 100).toFixed(1)}%</td>
            </tr>`;
    };

    const visibleRows = allFlakyTests
      .slice(0, LEADERBOARD_VISIBLE_ROWS)
      .map((e, i) => flakyRowHtml(e, i + 1))
      .join("\n          ");
    const hiddenEntries = allFlakyTests.slice(LEADERBOARD_VISIBLE_ROWS);
    const hiddenBlock =
      hiddenEntries.length > 0
        ? `<details>
        <summary>Show ${hiddenEntries.length} more...</summary>
        <table>
          <thead><tr><th>Rank</th><th>Test Name</th><th>Spec File</th><th>Failures</th><th>Appearances</th><th>Fail %</th></tr></thead>
          <tbody>
            ${hiddenEntries.map((e, i) => flakyRowHtml(e, LEADERBOARD_VISIBLE_ROWS + i + 1)).join("\n            ")}
          </tbody>
        </table>
      </details>`
        : "";

    flakySection = `<table>
      <thead><tr><th>Rank</th><th>Test Name</th><th>Spec File</th><th>Failures</th><th>Appearances</th><th>Fail %</th></tr></thead>
      <tbody>
        ${visibleRows}
      </tbody>
    </table>
    ${hiddenBlock}`;
  }

  // Detail sections - rows grouped by test name, then sorted by date within each group
  const detailSections = sortedSpecs
    .map((specFile) => {
      const shortName = specFile.replace("src/", "").replace(".spec.ts", "");

      // Collect all (date, test) pairs
      const entries = runResults.flatMap((r) => {
        const spec = r.specResults.find((s) => s.specFile === specFile);
        if (!spec || spec.tests.length === 0) return [];
        const date = r.run.startedAt.split("T")[0];
        return spec.tests.map((t) => ({ date, test: t }));
      });

      // Group by test name, preserving insertion order of first occurrence
      const groups = new Map<string, { date: string; durationMs: number; status: string }[]>();
      for (const { date, test } of entries) {
        if (!groups.has(test.name)) groups.set(test.name, []);
        groups.get(test.name)!.push({ date, durationMs: test.durationMs, status: test.status });
      }

      const rows = [...groups.entries()]
        .flatMap(([name, runs]) =>
          runs.map(
            ({ date, durationMs, status }) =>
              `<tr class="${status === "skipped" ? "skipped" : ""}">
                <td>${date}</td>
                <td style="text-align:left">${name}</td>
                <td>${status === "skipped" ? "-" : durationMs}</td>
                <td>${status}</td>
              </tr>`,
          ),
        )
        .join("\n            ");

      return `<details>
        <summary>${shortName}</summary>
        <table class="detail-table">
          <thead><tr><th>Date</th><th>Test Name</th><th>Duration (ms)</th><th>Status</th></tr></thead>
          <tbody>
            ${rows}
          </tbody>
        </table>
      </details>`;
    })
    .join("\n      ");

  const html = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>E2E CI Runtime Report</title>
  <style>${css}</style>
</head>
<body>
  <h1>E2E CI Runtime Report</h1>
  <div class="header-stats">
    <div class="stat"><strong>${totalRuns}</strong> runs</div>
    <div class="stat"><strong>${passedRuns}</strong> passed</div>
    <div class="stat"><strong>${failedRuns}</strong> failed</div>
    <div class="stat">Period: ${dateRange}</div>
    <div class="stat">Generated: ${new Date().toISOString()}</div>
  </div>

  <h2>Summary</h2>
  <table>
    <thead><tr><th>Spec File</th><th>Median (s)</th><th>Min (s)</th><th>Max (s)</th><th>Runs</th><th>Failures</th><th>Fail %</th></tr></thead>
    <tbody>
      ${totalRunSummaryRow}
      ${summaryRows}
    </tbody>
  </table>

  <h2>Top Flaky Tests</h2>
  ${flakySection}

  <h2>Timeline</h2>
  <div class="timeline-container">
    <table>
      <thead><tr><th>Spec File</th>${timelineHeaders}</tr></thead>
      <tbody>
        ${totalRunRow}
        ${timelineRows}
      </tbody>
    </table>
  </div>

  <h2>Test Case Details</h2>
  ${detailSections}
</body>
</html>`;

  return { html, summary };
}

async function main(): Promise<void> {
  const { days } = parseArgs();
  const token = process.env.GITHUB_TOKEN;
  if (!token) {
    console.error("GITHUB_TOKEN environment variable is required.");
    process.exit(1);
  }

  const octokit = new Octokit({ auth: token });
  const timeoutEligibleSpecFiles = discoverTimeoutEligibleSpecFiles();

  console.log(`Generating E2E runtime report for the last ${days} days...`);

  // Phase 1: Fetch
  const runsWithLogs = await fetchE2eRuns(octokit, days);
  if (runsWithLogs.length === 0) {
    console.log("No qualifying E2E runs found. Exiting.");
    // Write a sentinel so downstream workflow steps (cat summary, Slack post) don't fail
    // with "No such file or directory" when there's nothing to report.
    const emptySummary: ReportSummary = {
      totalRuns: 0,
      passedRuns: 0,
      failedRuns: 0,
      dateRange: "N/A",
      medianRunDurationSeconds: 0,
      maxRunDurationSeconds: 0,
      topFlakyTests: [],
      flakySlackText: "None detected this period.",
    };
    writeFileSync("e2e-runtime-report-summary.json", JSON.stringify(emptySummary) + "\n");
    process.exit(0);
  }

  // Phase 2: Parse
  const runResults: E2eRunResult[] = runsWithLogs.map(({ run, rawLog }) => ({
    run,
    specResults: parseJestLog(rawLog, run.jobConclusion, timeoutEligibleSpecFiles),
  }));

  // Sort by date ascending
  runResults.sort((a, b) => new Date(a.run.startedAt).getTime() - new Date(b.run.startedAt).getTime());

  console.log(`Parsed ${runResults.length} runs.`);

  for (const result of runResults) {
    const specCount = result.specResults.length;
    const date = result.run.startedAt.split("T")[0];
    console.log(`  ${date} run ${result.run.runId}: ${specCount} spec files, conclusion=${result.run.jobConclusion}`);
  }

  // Phase 3: Render
  const { html, summary } = renderHtmlReport(runResults);

  writeFileSync("e2e-runtime-report.html", html);
  writeFileSync("e2e-runtime-report-summary.json", JSON.stringify(summary) + "\n");

  console.log(`Report written to e2e-runtime-report.html`);
  console.log(`Summary: ${JSON.stringify(summary)}`);

  console.log("Done.");
}

main().catch((err) => {
  console.error("Fatal error:", err);
  process.exit(1);
});
