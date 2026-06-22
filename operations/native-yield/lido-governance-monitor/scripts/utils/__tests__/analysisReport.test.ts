import { describe, expect, it } from "@jest/globals";

import { computeEffectiveRisk, extractAnalysisEntriesFromLog, generateAnalysisReportHtml } from "../analysisReport.js";

const LOG_PREFIX = {
  RESPONSE: "message=AI response text content | class=ClaudeAIClient textContent=",
  COMPLETED: "message=AI analysis completed | class=ClaudeAIClient proposalTitle=",
} as const;

describe("analysisReport", () => {
  describe("computeEffectiveRisk", () => {
    it("rounds riskScore * confidence / 100", () => {
      // Arrange
      const riskScore = 85;
      const confidence = 75;

      // Act
      const result = computeEffectiveRisk(riskScore, confidence);

      // Assert
      expect(result).toBe(64);
    });
  });

  describe("extractAnalysisEntriesFromLog", () => {
    it("extracts fenced and inline JSON analysis blocks and computes effective risk", () => {
      // Arrange
      const logText = [
        `time=2026-03-05T10:40:59.332Z level=DEBUG ${LOG_PREFIX.RESPONSE}\`\`\`json`,
        "{",
        '  "riskScore": 50,',
        '  "confidence": 25,',
        '  "proposalType": "onchain_vote"',
        "}",
        "```",
        `time=2026-03-05T10:40:59.333Z level=DEBUG ${LOG_PREFIX.COMPLETED}LDO Contract vote 28 riskScore=50`,
        `time=2026-03-05T10:41:00.332Z level=DEBUG ${LOG_PREFIX.RESPONSE}{`,
        '  "riskScore": 35,',
        '  "confidence": 45,',
        '  "proposalType": "onchain_vote"',
        "}",
        `time=2026-03-05T10:41:00.333Z level=DEBUG ${LOG_PREFIX.COMPLETED}LDO Contract vote 29 riskScore=35`,
      ].join("\n");

      // Act
      const result = extractAnalysisEntriesFromLog(logText);

      // Assert
      expect(result).toHaveLength(2);

      expect(result[0]).toEqual(
        expect.objectContaining({
          index: 1,
          timestamp: "2026-03-05T10:40:59.332Z",
          proposalTitle: "LDO Contract vote 28",
          status: "parsed",
          computedEffectiveRisk: 13,
          loggedRiskScore: 50,
        }),
      );

      expect(result[1]).toEqual(
        expect.objectContaining({
          index: 2,
          proposalTitle: "LDO Contract vote 29",
          status: "parsed",
          computedEffectiveRisk: 16,
          loggedRiskScore: 35,
        }),
      );
    });

    it("strips ANSI escape codes and extracts entries from color-coded logs", () => {
      // Arrange - simulates mainnet log format with ANSI color wrapping
      const ESC = "\x1b";
      const logText = [
        `${ESC}[34mtime=2026-03-05T12:37:15.948Z level=DEBUG ${LOG_PREFIX.RESPONSE}\`\`\`json${ESC}[39m`,
        `${ESC}[34m{${ESC}[39m`,
        `${ESC}[34m  "riskScore": 50,${ESC}[39m`,
        `${ESC}[34m  "confidence": 25,${ESC}[39m`,
        `${ESC}[34m  "proposalType": "onchain_vote"${ESC}[39m`,
        `${ESC}[34m}${ESC}[39m`,
        `${ESC}[34m\`\`\`${ESC}[39m`,
        `${ESC}[34mtime=2026-03-05T12:37:15.949Z level=DEBUG ${LOG_PREFIX.COMPLETED}LDO Contract vote 28 riskScore=50${ESC}[39m`,
      ].join("\n");

      // Act
      const result = extractAnalysisEntriesFromLog(logText);

      // Assert
      expect(result).toHaveLength(1);
      expect(result[0]).toEqual(
        expect.objectContaining({
          index: 1,
          timestamp: "2026-03-05T12:37:15.948Z",
          proposalTitle: "LDO Contract vote 28",
          status: "parsed",
          riskScore: 50,
          confidence: 25,
          computedEffectiveRisk: 13,
          loggedRiskScore: 50,
        }),
      );
    });

    it("marks entry as invalid_json when textContent cannot be parsed", () => {
      // Arrange
      const logText = [
        `time=2026-03-05T10:50:00.000Z level=DEBUG ${LOG_PREFIX.RESPONSE}\`\`\`json`,
        "{",
        '  "riskScore": 50,',
        '  "confidence": "not-a-number"',
        "}",
        "```",
      ].join("\n");

      // Act
      const result = extractAnalysisEntriesFromLog(logText);

      // Assert
      expect(result).toHaveLength(1);
      expect(result[0]).toEqual(
        expect.objectContaining({
          status: "invalid_json",
          parseError: "Parsed JSON is missing numeric riskScore/confidence",
          computedEffectiveRisk: null,
          proposalTitle: null,
        }),
      );
    });
  });

  describe("generateAnalysisReportHtml", () => {
    it("renders clear delimiters and effective risk details for each entry", () => {
      // Arrange
      const logText = [
        `time=2026-03-05T10:40:59.332Z level=DEBUG ${LOG_PREFIX.RESPONSE}\`\`\`json`,
        "{",
        '  "riskScore": 50,',
        '  "confidence": 25',
        "}",
        "```",
      ].join("\n");
      const entries = extractAnalysisEntriesFromLog(logText);

      // Act
      const html = generateAnalysisReportHtml(entries, "run-hoodi.log");

      // Assert
      expect(html).toContain("Lido Governance Monitor - AI Analysis Report");
      expect(html).toContain("<strong>Effective Risk:</strong> 13/100");
      expect(html).toContain('class="analysis-delimiter"');
      expect(html).toContain("Analysis #1");
      expect(html).toContain("run-hoodi.log");
    });
  });
});
