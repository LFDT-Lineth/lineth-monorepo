import Anthropic from "@anthropic-ai/sdk";
import { jest, describe, it, expect, beforeEach } from "@jest/globals";

import { NativeYieldInvariant } from "../../core/entities/Assessment.js";
import { ILidoGovernanceMonitorLogger } from "../../utils/logging/index.js";
import { ClaudeAIClient } from "../ClaudeAIClient.js";

const createLoggerMock = (): jest.Mocked<ILidoGovernanceMonitorLogger> => ({
  name: "test-logger",
  debug: jest.fn(),
  error: jest.fn(),
  info: jest.fn(),
  warn: jest.fn(),
  critical: jest.fn(),
});

const TEST_SYSTEM_PROMPT = `You are a security analyst.

Respond with valid JSON.`;

describe("ClaudeAIClient", () => {
  let client: ClaudeAIClient;
  let logger: jest.Mocked<ILidoGovernanceMonitorLogger>;
  let mockAnthropicClient: { messages: { create: jest.Mock } };

  beforeEach(() => {
    logger = createLoggerMock();
    mockAnthropicClient = { messages: { create: jest.fn() } };
    client = new ClaudeAIClient(
      logger,
      mockAnthropicClient as unknown as Anthropic,
      "claude-sonnet-4-20250514",
      TEST_SYSTEM_PROMPT,
      4096, // maxOutputTokens
      700000, // maxProposalChars
    );
  });

  describe("analyzeProposal", () => {
    const createLLMOutput = (overrides = {}) => ({
      riskScore: 75,
      confidence: 85,
      proposalType: "discourse" as const,
      impactTypes: ["technical"] as const,
      affectedComponents: ["StakingVault"] as const,
      whatChanged: "Contract upgrade to v2",
      nativeYieldInvariantsAtRisk: [NativeYieldInvariant.VALID_YIELD_REPORTING] as const,
      nativeYieldImpact: ["May affect withdrawal mechanics"],
      supportingQuotes: ["The upgrade will modify..."],
      keyUnknowns: [],
      ...overrides,
    });

    const createAnalysisRequest = (overrides = {}) => ({
      proposalTitle: "Test Proposal",
      proposalText: "Proposal content",
      proposalUrl: "https://example.com",
      proposalType: "discourse" as const,
      ...overrides,
    });

    it("returns valid assessment when AI response is well-formed", async () => {
      // Arrange
      const llmOutput = createLLMOutput();
      mockAnthropicClient.messages.create.mockResolvedValue({
        content: [{ type: "text", text: JSON.stringify(llmOutput) }],
      });

      // Act
      const result = await client.analyzeProposal(createAnalysisRequest());

      // Assert
      expect(result).toEqual({
        ...llmOutput,
        riskLevel: "high",
        recommendedAction: "escalate",
        urgency: "urgent",
      });
      expect(logger.debug).toHaveBeenCalledWith("AI analysis completed", expect.any(Object));
    });

    it("extracts JSON from response with surrounding text", async () => {
      // Arrange
      const llmOutput = createLLMOutput({
        riskScore: 50,
        confidence: 70,
        impactTypes: ["economic"],
        whatChanged: "Fee structure change",
        nativeYieldImpact: ["May impact yields"],
        keyUnknowns: ["Exact fee impact unclear"],
      });
      mockAnthropicClient.messages.create.mockResolvedValue({
        content: [{ type: "text", text: `Here's my analysis:\n${JSON.stringify(llmOutput)}\nEnd of analysis.` }],
      });

      // Act
      const result = await client.analyzeProposal(createAnalysisRequest());

      // Assert
      expect(result).toEqual({
        ...llmOutput,
        riskLevel: "medium",
        recommendedAction: "monitor",
        urgency: "none",
      });
    });

    it("returns undefined when AI response fails schema validation", async () => {
      // Arrange
      const invalidAssessment = { riskScore: 150, impactTypes: ["invalid-type"] };
      mockAnthropicClient.messages.create.mockResolvedValue({
        content: [{ type: "text", text: JSON.stringify(invalidAssessment) }],
      });

      // Act
      const result = await client.analyzeProposal(createAnalysisRequest());

      // Assert
      expect(result).toBeUndefined();
      expect(logger.error).toHaveBeenCalledWith("AI response failed schema validation", expect.any(Object));
    });

    it("returns undefined when response has no JSON", async () => {
      // Arrange
      mockAnthropicClient.messages.create.mockResolvedValue({
        content: [{ type: "text", text: "I cannot analyze this proposal." }],
      });

      // Act
      const result = await client.analyzeProposal(createAnalysisRequest());

      // Assert
      expect(result).toBeUndefined();
      expect(logger.error).toHaveBeenCalledWith("No JSON found in AI response");
    });

    it("returns undefined on API failure and logs critical", async () => {
      // Arrange
      mockAnthropicClient.messages.create.mockRejectedValue(new Error("API rate limit exceeded"));

      // Act
      const result = await client.analyzeProposal(createAnalysisRequest());

      // Assert
      expect(result).toBeUndefined();
      expect(logger.critical).toHaveBeenCalledWith("AI analysis failed", expect.any(Object));
    });

    it("returns undefined when response has no text content", async () => {
      // Arrange
      mockAnthropicClient.messages.create.mockResolvedValue({
        content: [],
      });

      // Act
      const result = await client.analyzeProposal(createAnalysisRequest());

      // Assert
      expect(result).toBeUndefined();
      expect(logger.error).toHaveBeenCalledWith("AI response missing text content");
    });

    it("returns undefined when JSON is malformed", async () => {
      // Arrange
      mockAnthropicClient.messages.create.mockResolvedValue({
        content: [{ type: "text", text: "{ invalid json }" }],
      });

      // Act
      const result = await client.analyzeProposal(createAnalysisRequest());

      // Assert
      expect(result).toBeUndefined();
      expect(logger.error).toHaveBeenCalledWith("Failed to parse AI response as JSON", expect.any(Object));
    });

    it("truncates proposal text when it exceeds max character limit", async () => {
      // Arrange
      const largeText = "a".repeat(800000); // Exceeds 700000 char limit
      const validAssessment = createLLMOutput();
      mockAnthropicClient.messages.create.mockResolvedValue({
        content: [{ type: "text", text: JSON.stringify(validAssessment) }],
      });

      const request = createAnalysisRequest({
        proposalText: largeText,
      });

      // Act
      const result = await client.analyzeProposal(request);

      // Assert
      expect(result).toBeDefined();
      expect(mockAnthropicClient.messages.create).toHaveBeenCalledWith(
        expect.objectContaining({
          messages: [
            expect.objectContaining({
              content: expect.stringContaining("a".repeat(700000)), // Truncated to 700000
            }),
          ],
        }),
      );
    });

    describe("confidence validation", () => {
      const createMockAIResponse = (overrides: Partial<ReturnType<typeof createLLMOutput>>) => {
        const assessment = createLLMOutput(overrides);
        return {
          content: [{ type: "text" as const, text: JSON.stringify(assessment) }],
        };
      };

      it("rejects float confidence (0.0-1.0 format)", async () => {
        // Arrange
        const mockResponse = createMockAIResponse({ confidence: 0.85 as any }); // Float instead of integer
        mockAnthropicClient.messages.create.mockResolvedValue(mockResponse);

        // Act
        const result = await client.analyzeProposal(createAnalysisRequest());

        // Assert
        expect(result).toBeUndefined();
        expect(logger.error).toHaveBeenCalledWith(
          "AI response failed schema validation",
          expect.objectContaining({
            errors: expect.arrayContaining([
              expect.objectContaining({
                path: ["confidence"],
                message: expect.stringContaining("Expected integer"),
              }),
            ]),
          }),
        );
      });

      it("accepts confidence at boundary values", async () => {
        // Arrange - Test minimum value (needs keyUnknowns since confidence < 80)
        const mockResponseMin = createMockAIResponse({ confidence: 0, keyUnknowns: ["Insufficient data"] });
        mockAnthropicClient.messages.create.mockResolvedValue(mockResponseMin);

        // Act
        const resultMin = await client.analyzeProposal(createAnalysisRequest());

        // Assert
        expect(resultMin).toBeDefined();
        expect(resultMin?.confidence).toBe(0);

        // Arrange - Test maximum value
        const mockResponseMax = createMockAIResponse({ confidence: 100 });
        mockAnthropicClient.messages.create.mockResolvedValue(mockResponseMax);

        // Act
        const resultMax = await client.analyzeProposal(createAnalysisRequest());

        // Assert
        expect(resultMax).toBeDefined();
        expect(resultMax?.confidence).toBe(100);
      });

      it("rejects out-of-range confidence values", async () => {
        // Arrange - Test above maximum
        const mockResponseHigh = createMockAIResponse({ confidence: 101 as any });
        mockAnthropicClient.messages.create.mockResolvedValue(mockResponseHigh);

        // Act
        const resultHigh = await client.analyzeProposal(createAnalysisRequest());

        // Assert
        expect(resultHigh).toBeUndefined();

        // Arrange - Test below minimum
        const mockResponseLow = createMockAIResponse({ confidence: -1 as any });
        mockAnthropicClient.messages.create.mockResolvedValue(mockResponseLow);

        // Act
        const resultLow = await client.analyzeProposal(createAnalysisRequest());

        // Assert
        expect(resultLow).toBeUndefined();
      });

      it("rejects non-integer confidence (fractional values)", async () => {
        // Arrange
        const mockResponse = createMockAIResponse({ confidence: 75.5 as any });
        mockAnthropicClient.messages.create.mockResolvedValue(mockResponse);

        // Act
        const result = await client.analyzeProposal(createAnalysisRequest());

        // Assert
        expect(result).toBeUndefined();
        expect(logger.error).toHaveBeenCalledWith(
          "AI response failed schema validation",
          expect.objectContaining({
            errors: expect.arrayContaining([
              expect.objectContaining({
                path: ["confidence"],
              }),
            ]),
          }),
        );
      });
    });

    describe("derived fields from riskScore", () => {
      const testDerivation = async (
        riskScore: number,
        expected: { riskLevel: string; recommendedAction: string; urgency: string },
      ) => {
        const keyUnknowns = riskScore < 80 ? ["Some unknown"] : [];
        const confidence = riskScore < 80 ? 70 : 85;
        const llmOutput = createLLMOutput({ riskScore, confidence, keyUnknowns });
        mockAnthropicClient.messages.create.mockResolvedValue({
          content: [{ type: "text", text: JSON.stringify(llmOutput) }],
        });
        const result = await client.analyzeProposal(createAnalysisRequest());
        expect(result).toBeDefined();
        expect(result?.riskLevel).toBe(expected.riskLevel);
        expect(result?.recommendedAction).toBe(expected.recommendedAction);
        expect(result?.urgency).toBe(expected.urgency);
      };

      it("riskScore 15 -> low / no-action / none", async () => {
        await testDerivation(15, { riskLevel: "low", recommendedAction: "no-action", urgency: "none" });
      });

      it("riskScore 45 -> medium / monitor / none", async () => {
        await testDerivation(45, { riskLevel: "medium", recommendedAction: "monitor", urgency: "none" });
      });

      it("riskScore 65 -> high / comment / routine", async () => {
        await testDerivation(65, { riskLevel: "high", recommendedAction: "comment", urgency: "routine" });
      });

      it("riskScore 75 -> high / escalate / urgent", async () => {
        await testDerivation(75, { riskLevel: "high", recommendedAction: "escalate", urgency: "urgent" });
      });

      it("riskScore 90 -> critical / escalate / critical", async () => {
        await testDerivation(90, { riskLevel: "critical", recommendedAction: "escalate", urgency: "critical" });
      });
    });

    describe("evidence requirements", () => {
      it("rejects empty supportingQuotes", async () => {
        // Arrange
        const llmOutput = createLLMOutput({ supportingQuotes: [] });
        mockAnthropicClient.messages.create.mockResolvedValue({
          content: [{ type: "text", text: JSON.stringify(llmOutput) }],
        });

        // Act
        const result = await client.analyzeProposal(createAnalysisRequest());

        // Assert
        expect(result).toBeUndefined();
        expect(logger.error).toHaveBeenCalledWith("AI response failed schema validation", expect.any(Object));
      });

      it("rejects empty keyUnknowns when confidence < 80", async () => {
        // Arrange
        const llmOutput = createLLMOutput({ confidence: 70, keyUnknowns: [] });
        mockAnthropicClient.messages.create.mockResolvedValue({
          content: [{ type: "text", text: JSON.stringify(llmOutput) }],
        });

        // Act
        const result = await client.analyzeProposal(createAnalysisRequest());

        // Assert
        expect(result).toBeUndefined();
        expect(logger.error).toHaveBeenCalledWith("AI response failed schema validation", expect.any(Object));
      });

      it("accepts empty keyUnknowns when confidence >= 80", async () => {
        // Arrange
        const llmOutput = createLLMOutput({ confidence: 80, keyUnknowns: [] });
        mockAnthropicClient.messages.create.mockResolvedValue({
          content: [{ type: "text", text: JSON.stringify(llmOutput) }],
        });

        // Act
        const result = await client.analyzeProposal(createAnalysisRequest());

        // Assert
        expect(result).toBeDefined();
      });
    });

    describe("input validation", () => {
      it("returns undefined for invalid URL format", async () => {
        // Arrange
        const request = createAnalysisRequest({
          proposalUrl: "not-a-valid-url",
        });

        // Act
        const result = await client.analyzeProposal(request);

        // Assert
        expect(result).toBeUndefined();
        expect(logger.error).toHaveBeenCalledWith(
          "Invalid analysis request",
          expect.objectContaining({
            errors: expect.arrayContaining([expect.objectContaining({ path: ["proposalUrl"] })]),
          }),
        );
        expect(mockAnthropicClient.messages.create).not.toHaveBeenCalled();
      });

      it("returns undefined for title exceeding 1000 characters", async () => {
        // Arrange
        const request = createAnalysisRequest({
          proposalTitle: "a".repeat(1001),
        });

        // Act
        const result = await client.analyzeProposal(request);

        // Assert
        expect(result).toBeUndefined();
        expect(logger.error).toHaveBeenCalledWith(
          "Invalid analysis request",
          expect.objectContaining({
            errors: expect.arrayContaining([expect.objectContaining({ path: ["proposalTitle"] })]),
          }),
        );
      });

      it("accepts valid request with 1000 character title", async () => {
        // Arrange
        const validAssessment = createLLMOutput();
        mockAnthropicClient.messages.create.mockResolvedValue({
          content: [{ type: "text", text: JSON.stringify(validAssessment) }],
        });

        const request = createAnalysisRequest({
          proposalTitle: "a".repeat(1000),
        });

        // Act
        const result = await client.analyzeProposal(request);

        // Assert
        expect(result).toBeDefined();
      });
    });
  });

  describe("getModelName", () => {
    it("returns the configured model name", () => {
      // Act & Assert
      expect(client.getModelName()).toBe("claude-sonnet-4-20250514");
    });
  });
});
