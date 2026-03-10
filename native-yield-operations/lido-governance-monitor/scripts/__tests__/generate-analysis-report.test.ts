import * as fs from "fs";
import * as os from "os";
import * as path from "path";

import { afterEach, beforeEach, describe, expect, it, jest } from "@jest/globals";

import { resolveOutputPath, runCli } from "../generate-analysis-report.js";

describe("generate-analysis-report script", () => {
  const LOG_MARKER = "message=AI response text content | class=ClaudeAIClient textContent=";

  let tempDir: string;
  let logSpy: jest.SpiedFunction<typeof console.log>;
  let errorSpy: jest.SpiedFunction<typeof console.error>;

  beforeEach(() => {
    tempDir = fs.mkdtempSync(path.join(os.tmpdir(), "lido-analysis-report-"));
    logSpy = jest.spyOn(console, "log").mockImplementation(() => undefined);
    errorSpy = jest.spyOn(console, "error").mockImplementation(() => undefined);
  });

  afterEach(() => {
    logSpy.mockRestore();
    errorSpy.mockRestore();
    fs.rmSync(tempDir, { recursive: true, force: true });
  });

  it("builds default output path next to input file", () => {
    // Arrange
    const inputPath = "/tmp/run-hoodi.log";

    // Act
    const result = resolveOutputPath(inputPath);

    // Assert
    expect(result).toBe(path.resolve("/tmp/run-hoodi.analysis-report.html"));
  });

  it("returns non-zero when input log path does not exist", () => {
    // Arrange
    const missingPath = path.join(tempDir, "missing.log");

    // Act
    const result = runCli(["node", "tsx", missingPath]);

    // Assert
    expect(result).toBe(1);
    expect(errorSpy).toHaveBeenCalledWith(expect.stringContaining("Input log file not found:"));
  });

  it("writes html report and returns zero for a valid log", () => {
    // Arrange
    const inputPath = path.join(tempDir, "run-hoodi.log");
    const outputPath = path.join(tempDir, "report.html");
    const logText = [
      `time=2026-03-05T10:40:59.332Z level=DEBUG ${LOG_MARKER}\`\`\`json`,
      "{",
      '  "riskScore": 50,',
      '  "confidence": 25',
      "}",
      "```",
      "time=2026-03-05T10:40:59.333Z level=DEBUG message=AI analysis completed | class=ClaudeAIClient proposalTitle=LDO Contract vote 55 riskScore=50",
    ].join("\n");
    fs.writeFileSync(inputPath, logText, "utf-8");

    // Act
    const result = runCli(["node", "tsx", inputPath, outputPath]);

    // Assert
    expect(result).toBe(0);
    expect(fs.existsSync(outputPath)).toBe(true);

    const html = fs.readFileSync(outputPath, "utf-8");
    expect(html).toContain("Lido Governance Monitor - AI Analysis Report");
    expect(html).toContain("LDO Contract vote 55");
    expect(html).toContain("<strong>Effective Risk:</strong> 13/100");
  });
});
