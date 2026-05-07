import { computeEffectiveRisk } from "../../src/utils/effectiveRisk.js";

export { computeEffectiveRisk };

const LOG_MARKER = {
  TIME_PREFIX: "time=",
  RESPONSE_TEXT: "message=AI response text content | class=ClaudeAIClient textContent=",
  ANALYSIS_COMPLETED: "message=AI analysis completed | class=ClaudeAIClient proposalTitle=",
} as const;

const ENTRY_STATUS = {
  PARSED: "parsed",
  INVALID_JSON: "invalid_json",
} as const;

const PARSE_ERROR = {
  JSON_NOT_FOUND: "No JSON object found in textContent",
  MISSING_NUMERIC_FIELDS: "Parsed JSON is missing numeric riskScore/confidence",
} as const;

const HTML_LABEL = {
  TITLE: "Lido Governance Monitor - AI Analysis Report",
  SUMMARY: "Summary",
  RAW_JSON: "Raw AI JSON",
  WHAT_CHANGED: "What Changed",
  NATIVE_YIELD_IMPACT: "Native Yield Impact",
  SUPPORTING_QUOTES: "Supporting Quotes",
  KEY_UNKNOWNS: "Key Unknowns",
  PARSED: "Parsed",
  INVALID_JSON: "Invalid JSON",
  NONE: "(none)",
  NOT_AVAILABLE: "n/a",
} as const;

export type AnalysisEntryStatus = (typeof ENTRY_STATUS)[keyof typeof ENTRY_STATUS];

type LLMOutputWithScores = Record<string, unknown> & {
  riskScore: number;
  confidence: number;
};

export interface AnalysisReportEntry {
  index: number;
  timestamp: string | null;
  proposalTitle: string | null;
  loggedRiskScore: number | null;
  riskScore: number | null;
  confidence: number | null;
  computedEffectiveRisk: number | null;
  status: AnalysisEntryStatus;
  parseError: string | null;
  rawJsonText: string;
  analysis: Record<string, unknown> | null;
}

interface BlockCaptureResult {
  textContent: string;
  nextLineIndex: number;
}

interface CompletionMetadata {
  proposalTitle: string;
  loggedRiskScore: number | null;
}

interface ParseResult {
  status: AnalysisEntryStatus;
  parseError: string | null;
  riskScore: number | null;
  confidence: number | null;
  computedEffectiveRisk: number | null;
  analysis: Record<string, unknown> | null;
}

const ANSI_ESCAPE_RE = /\x1b\[[0-9;]*m/g;

export function stripAnsiCodes(text: string): string {
  return text.replace(ANSI_ESCAPE_RE, "");
}

export function extractAnalysisEntriesFromLog(logText: string): AnalysisReportEntry[] {
  const lines = stripAnsiCodes(logText).split(/\r?\n/);
  const entries: AnalysisReportEntry[] = [];

  let lineIndex = 0;
  while (lineIndex < lines.length) {
    const line = lines[lineIndex];
    const markerIndex = line.indexOf(LOG_MARKER.RESPONSE_TEXT);
    if (markerIndex === -1) {
      lineIndex += 1;
      continue;
    }

    const timestamp = extractTimestamp(line);
    const initialTextContent = line.slice(markerIndex + LOG_MARKER.RESPONSE_TEXT.length);
    const capturedBlock = captureTextContentBlock(lines, lineIndex, initialTextContent);
    const completionMetadata = findCompletionMetadata(lines, capturedBlock.nextLineIndex);
    const parseResult = parseAnalysisJson(capturedBlock.textContent);

    entries.push({
      index: entries.length + 1,
      timestamp,
      proposalTitle: completionMetadata?.proposalTitle ?? null,
      loggedRiskScore: completionMetadata?.loggedRiskScore ?? null,
      riskScore: parseResult.riskScore,
      confidence: parseResult.confidence,
      computedEffectiveRisk: parseResult.computedEffectiveRisk,
      status: parseResult.status,
      parseError: parseResult.parseError,
      rawJsonText: capturedBlock.textContent,
      analysis: parseResult.analysis,
    });

    lineIndex = capturedBlock.nextLineIndex;
  }

  return entries;
}

export function generateAnalysisReportHtml(entries: AnalysisReportEntry[], sourceLabel: string): string {
  const parsedCount = entries.filter((entry) => entry.status === ENTRY_STATUS.PARSED).length;
  const invalidCount = entries.length - parsedCount;
  const averageEffectiveRisk =
    parsedCount === 0
      ? HTML_LABEL.NOT_AVAILABLE
      : String(
          Math.round(
            entries
              .filter((entry) => entry.computedEffectiveRisk !== null)
              .reduce((total, entry) => total + (entry.computedEffectiveRisk ?? 0), 0) / parsedCount,
          ),
        );

  const renderedEntries = entries
    .map((entry) => {
      const proposalTitle = entry.proposalTitle ?? HTML_LABEL.NOT_AVAILABLE;
      const timestamp = entry.timestamp ?? HTML_LABEL.NOT_AVAILABLE;
      const loggedRiskScore = entry.loggedRiskScore ?? HTML_LABEL.NOT_AVAILABLE;
      const riskScore = entry.riskScore ?? HTML_LABEL.NOT_AVAILABLE;
      const confidence = entry.confidence ?? HTML_LABEL.NOT_AVAILABLE;
      const effectiveRisk = entry.computedEffectiveRisk ?? HTML_LABEL.NOT_AVAILABLE;
      const statusClass = entry.status === ENTRY_STATUS.PARSED ? "status-ok" : "status-error";
      const statusLabel = entry.status === ENTRY_STATUS.PARSED ? HTML_LABEL.PARSED : HTML_LABEL.INVALID_JSON;
      const whatChanged = getStringField(entry.analysis, "whatChanged");
      const nativeYieldImpact = getStringArrayField(entry.analysis, "nativeYieldImpact");
      const supportingQuotes = getStringArrayField(entry.analysis, "supportingQuotes");
      const keyUnknowns = getStringArrayField(entry.analysis, "keyUnknowns");
      const parseErrorRow = entry.parseError
        ? `<p class="parse-error"><strong>Parse error:</strong> ${escapeHtml(entry.parseError)}</p>`
        : "";

      return [
        '<article class="analysis-card">',
        `<header class="analysis-header"><h2>Analysis #${entry.index}</h2><span class="entry-status ${statusClass}">${escapeHtml(statusLabel)}</span></header>`,
        '<div class="analysis-meta">',
        `<p><strong>Timestamp:</strong> ${escapeHtml(timestamp)}</p>`,
        `<p><strong>Proposal:</strong> ${escapeHtml(proposalTitle)}</p>`,
        "</div>",
        '<div class="metric-grid">',
        `<p><strong>riskScore:</strong> ${escapeHtml(String(riskScore))}</p>`,
        `<p><strong>confidence:</strong> ${escapeHtml(String(confidence))}</p>`,
        `<p><strong>Effective Risk:</strong> ${escapeHtml(String(effectiveRisk))}/100</p>`,
        `<p><strong>Logged riskScore:</strong> ${escapeHtml(String(loggedRiskScore))}</p>`,
        "</div>",
        parseErrorRow,
        renderSection(HTML_LABEL.WHAT_CHANGED, whatChanged),
        renderListSection(HTML_LABEL.NATIVE_YIELD_IMPACT, nativeYieldImpact),
        renderListSection(HTML_LABEL.SUPPORTING_QUOTES, supportingQuotes),
        renderListSection(HTML_LABEL.KEY_UNKNOWNS, keyUnknowns),
        `<details><summary>${HTML_LABEL.RAW_JSON}</summary><pre>${escapeHtml(entry.rawJsonText)}</pre></details>`,
        "</article>",
        '<hr class="analysis-delimiter" />',
      ].join("");
    })
    .join("");

  return [
    "<!doctype html>",
    '<html lang="en">',
    "<head>",
    '<meta charset="utf-8" />',
    '<meta name="viewport" content="width=device-width, initial-scale=1" />',
    `<title>${HTML_LABEL.TITLE}</title>`,
    "<style>",
    "body { margin: 0; font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace; background: #f4f6fb; color: #121826; }",
    ".container { max-width: 1200px; margin: 0 auto; padding: 24px; }",
    "h1 { margin: 0 0 16px 0; font-size: 28px; }",
    ".summary { background: #ffffff; border: 1px solid #d8e0f0; border-radius: 10px; padding: 16px; margin-bottom: 20px; }",
    ".summary p { margin: 6px 0; }",
    ".analysis-card { background: #ffffff; border: 1px solid #d8e0f0; border-radius: 10px; padding: 16px; }",
    ".analysis-header { display: flex; justify-content: space-between; align-items: center; gap: 12px; }",
    ".analysis-header h2 { margin: 0; font-size: 19px; }",
    ".entry-status { display: inline-block; border-radius: 999px; padding: 4px 10px; font-size: 12px; font-weight: 700; }",
    ".status-ok { background: #e7f7ef; color: #166534; }",
    ".status-error { background: #fde8e8; color: #991b1b; }",
    ".analysis-meta { margin: 10px 0; }",
    ".analysis-meta p { margin: 4px 0; }",
    ".metric-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); gap: 10px; margin: 14px 0; }",
    ".metric-grid p { margin: 0; background: #f8fafc; border: 1px solid #e2e8f0; border-radius: 8px; padding: 8px 10px; }",
    ".section { margin-top: 14px; }",
    ".section h3 { margin: 0 0 6px 0; font-size: 15px; }",
    ".section p { margin: 0; white-space: pre-wrap; }",
    ".section ul { margin: 0; padding-left: 20px; }",
    ".section li { margin: 0 0 4px 0; }",
    ".parse-error { margin: 0; color: #991b1b; }",
    "details { margin-top: 14px; }",
    "pre { white-space: pre-wrap; background: #0f172a; color: #e2e8f0; border-radius: 8px; padding: 12px; overflow-x: auto; }",
    ".analysis-delimiter { border: none; border-top: 3px dashed #94a3b8; margin: 18px 0; }",
    "@media (max-width: 640px) { .container { padding: 14px; } .analysis-header { align-items: flex-start; flex-direction: column; } }",
    "</style>",
    "</head>",
    "<body>",
    '<div class="container">',
    `<h1>${HTML_LABEL.TITLE}</h1>`,
    `<section class="summary"><h2>${HTML_LABEL.SUMMARY}</h2><p><strong>Source log:</strong> ${escapeHtml(sourceLabel)}</p><p><strong>Total analyses:</strong> ${entries.length}</p><p><strong>Parsed:</strong> ${parsedCount}</p><p><strong>Invalid:</strong> ${invalidCount}</p><p><strong>Average effectiveRisk:</strong> ${escapeHtml(averageEffectiveRisk)}/100</p></section>`,
    renderedEntries,
    "</div>",
    "</body>",
    "</html>",
  ].join("");
}

function captureTextContentBlock(lines: string[], startIndex: number, initialTextContent: string): BlockCaptureResult {
  const normalizedInitial = initialTextContent.trim();
  if (normalizedInitial.startsWith("```")) {
    const blockLines: string[] = [initialTextContent];
    let cursor = startIndex + 1;
    while (cursor < lines.length) {
      blockLines.push(lines[cursor]);
      if (lines[cursor].trim() === "```") {
        cursor += 1;
        break;
      }
      cursor += 1;
    }
    return {
      textContent: normalizeTextContent(blockLines.join("\n")),
      nextLineIndex: cursor,
    };
  }

  const blockLines: string[] = [initialTextContent];
  let cursor = startIndex + 1;
  while (cursor < lines.length && !isLogLine(lines[cursor])) {
    blockLines.push(lines[cursor]);
    cursor += 1;
  }

  return {
    textContent: normalizeTextContent(blockLines.join("\n")),
    nextLineIndex: cursor,
  };
}

function normalizeTextContent(rawTextContent: string): string {
  const normalized = rawTextContent.trim();
  if (!normalized.startsWith("```")) {
    return normalized;
  }

  const lines = normalized.split(/\r?\n/);
  if (lines.length <= 1) return normalized;

  const bodyStartIndex = 1;
  const bodyEndIndex = lines[lines.length - 1].trim() === "```" ? lines.length - 1 : lines.length;
  return lines.slice(bodyStartIndex, bodyEndIndex).join("\n").trim();
}

function findCompletionMetadata(lines: string[], startIndex: number): CompletionMetadata | null {
  let cursor = startIndex;
  while (cursor < lines.length) {
    const line = lines[cursor];
    if (line.includes(LOG_MARKER.RESPONSE_TEXT)) {
      return null;
    }
    if (line.includes(LOG_MARKER.ANALYSIS_COMPLETED)) {
      const completionPattern = /proposalTitle=(.*) riskScore=(\d+)\s*$/;
      const match = line.match(completionPattern);
      if (!match) return null;
      return {
        proposalTitle: match[1].trim(),
        loggedRiskScore: Number.parseInt(match[2], 10),
      };
    }
    cursor += 1;
  }
  return null;
}

function parseAnalysisJson(textContent: string): ParseResult {
  const jsonMatch = textContent.match(/\{[\s\S]*\}/);
  if (!jsonMatch) {
    return {
      status: ENTRY_STATUS.INVALID_JSON,
      parseError: PARSE_ERROR.JSON_NOT_FOUND,
      riskScore: null,
      confidence: null,
      computedEffectiveRisk: null,
      analysis: null,
    };
  }

  try {
    const parsed = JSON.parse(jsonMatch[0]) as Record<string, unknown>;
    if (typeof parsed.riskScore !== "number" || typeof parsed.confidence !== "number") {
      return {
        status: ENTRY_STATUS.INVALID_JSON,
        parseError: PARSE_ERROR.MISSING_NUMERIC_FIELDS,
        riskScore: null,
        confidence: null,
        computedEffectiveRisk: null,
        analysis: parsed,
      };
    }

    const typedParsed = parsed as LLMOutputWithScores;
    return {
      status: ENTRY_STATUS.PARSED,
      parseError: null,
      riskScore: typedParsed.riskScore,
      confidence: typedParsed.confidence,
      computedEffectiveRisk: computeEffectiveRisk(typedParsed.riskScore, typedParsed.confidence),
      analysis: parsed,
    };
  } catch (error) {
    const parseMessage = error instanceof Error ? error.message : "Unknown JSON parse error";
    return {
      status: ENTRY_STATUS.INVALID_JSON,
      parseError: parseMessage,
      riskScore: null,
      confidence: null,
      computedEffectiveRisk: null,
      analysis: null,
    };
  }
}

function extractTimestamp(line: string): string | null {
  if (!line.startsWith(LOG_MARKER.TIME_PREFIX)) return null;
  const timestampMatch = line.match(/^time=([^ ]+)/);
  return timestampMatch ? timestampMatch[1] : null;
}

function isLogLine(line: string): boolean {
  return line.startsWith(LOG_MARKER.TIME_PREFIX);
}

function renderSection(title: string, body: string): string {
  return `<section class="section"><h3>${escapeHtml(title)}</h3><p>${escapeHtml(body)}</p></section>`;
}

function renderListSection(title: string, values: string[]): string {
  if (values.length === 0) {
    return `<section class="section"><h3>${escapeHtml(title)}</h3><p>${HTML_LABEL.NONE}</p></section>`;
  }
  const items = values.map((value) => `<li>${escapeHtml(value)}</li>`).join("");
  return `<section class="section"><h3>${escapeHtml(title)}</h3><ul>${items}</ul></section>`;
}

function getStringField(data: Record<string, unknown> | null, key: string): string {
  if (!data) return HTML_LABEL.NONE;
  const value = data[key];
  return typeof value === "string" && value.trim().length > 0 ? value : HTML_LABEL.NONE;
}

function getStringArrayField(data: Record<string, unknown> | null, key: string): string[] {
  if (!data) return [];
  const value = data[key];
  if (!Array.isArray(value)) return [];
  return value.filter((item): item is string => typeof item === "string");
}

function escapeHtml(value: string): string {
  return value.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;");
}
