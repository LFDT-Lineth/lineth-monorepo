import Anthropic from "@anthropic-ai/sdk";
import { z } from "zod";

import { IAIClient, AIAnalysisRequest } from "../core/clients/IAIClient.js";
import {
  Assessment,
  NativeYieldInvariant,
  RiskLevel,
  RecommendedAction,
  Urgency,
} from "../core/entities/Assessment.js";
import { ILidoGovernanceMonitorLogger } from "../utils/logging/index.js";

const LLMOutputSchema = z
  .object({
    riskScore: z.number().int().min(0).max(100),
    confidence: z.number().int().min(0).max(100),
    proposalType: z.enum(["discourse", "snapshot", "onchain_vote"]),
    impactTypes: z.array(z.enum(["economic", "technical", "operational", "governance-process"])),
    affectedComponents: z.array(
      z.enum(["StakingVault", "VaultHub", "LazyOracle", "OperatorGrid", "PredepositGuarantee", "Dashboard", "Other"]),
    ),
    whatChanged: z.string().min(1),
    nativeYieldInvariantsAtRisk: z.array(
      z.enum([
        NativeYieldInvariant.VALID_YIELD_REPORTING,
        NativeYieldInvariant.USER_PRINCIPAL_PROTECTION,
        NativeYieldInvariant.PAUSE_DEPOSITS,
        NativeYieldInvariant.OTHER,
      ]),
    ),
    nativeYieldImpact: z.array(z.string()).min(1),
    supportingQuotes: z.array(z.string()).min(1),
    keyUnknowns: z.array(z.string()),
  })
  .refine((data) => data.confidence >= 80 || data.keyUnknowns.length >= 1, {
    message: "keyUnknowns must have at least 1 entry when confidence < 80",
    path: ["keyUnknowns"],
  });

const AIAnalysisRequestSchema = z.object({
  proposalTitle: z.string().max(1000),
  proposalText: z.string(),
  proposalUrl: z.string().url(),
  proposalType: z.enum(["discourse", "snapshot", "onchain_vote"]),
});

function deriveRiskLevel(riskScore: number): RiskLevel {
  if (riskScore >= 81) return "critical";
  if (riskScore >= 61) return "high";
  if (riskScore >= 31) return "medium";
  return "low";
}

function deriveRecommendedAction(riskScore: number): RecommendedAction {
  if (riskScore >= 71) return "escalate";
  if (riskScore >= 51) return "comment";
  if (riskScore >= 21) return "monitor";
  return "no-action";
}

function deriveUrgency(riskScore: number): Urgency {
  if (riskScore >= 86) return "critical";
  if (riskScore >= 71) return "urgent";
  if (riskScore >= 51) return "routine";
  return "none";
}

export class ClaudeAIClient implements IAIClient {
  constructor(
    private readonly logger: ILidoGovernanceMonitorLogger,
    private readonly anthropicClient: Anthropic,
    private readonly modelName: string,
    private readonly systemPromptTemplate: string,
    private readonly maxOutputTokens: number,
    private readonly maxProposalChars: number,
  ) {}

  async analyzeProposal(request: AIAnalysisRequest): Promise<Assessment | undefined> {
    const validationResult = AIAnalysisRequestSchema.safeParse(request);
    if (!validationResult.success) {
      this.logger.error("Invalid analysis request", {
        errors: validationResult.error.errors,
      });
      return undefined;
    }

    const userPrompt = this.buildUserPrompt(request);

    try {
      const response = await this.anthropicClient.messages.create({
        model: this.modelName,
        max_tokens: this.maxOutputTokens,
        system: this.systemPromptTemplate,
        messages: [{ role: "user", content: userPrompt }],
      });

      const textContent = response.content.find((c) => c.type === "text");
      if (!textContent || textContent.type !== "text") {
        this.logger.error("AI response missing text content");
        return undefined;
      }

      this.logger.debug("AI response text content", { textContent: textContent.text });

      const parsed = this.parseAndValidate(textContent.text);
      if (!parsed) return undefined;

      this.logger.debug("AI analysis completed", {
        proposalTitle: request.proposalTitle,
        riskScore: parsed.riskScore,
      });
      return parsed;
    } catch (error) {
      this.logger.critical("AI analysis failed", { error });
      return undefined;
    }
  }

  getModelName(): string {
    return this.modelName;
  }

  private parseAndValidate(responseText: string): Assessment | undefined {
    try {
      const jsonMatch = responseText.match(/\{[\s\S]*\}/);
      if (!jsonMatch) {
        this.logger.error("No JSON found in AI response");
        return undefined;
      }
      const parsed = JSON.parse(jsonMatch[0]);
      const result = LLMOutputSchema.safeParse(parsed);
      if (!result.success) {
        this.logger.error("AI response failed schema validation", { errors: result.error.errors });
        return undefined;
      }
      const llmOutput = result.data;
      return {
        ...llmOutput,
        riskLevel: deriveRiskLevel(llmOutput.riskScore),
        recommendedAction: deriveRecommendedAction(llmOutput.riskScore),
        urgency: deriveUrgency(llmOutput.riskScore),
      };
    } catch (error) {
      this.logger.error("Failed to parse AI response as JSON", { error });
      return undefined;
    }
  }

  private buildUserPrompt(request: AIAnalysisRequest): string {
    const truncatedText = request.proposalText.substring(0, this.maxProposalChars);

    return `Analyze this Lido governance proposal:

Title: ${request.proposalTitle}
URL: ${request.proposalUrl}
Type: ${request.proposalType}

Content:
${truncatedText}`;
  }
}
