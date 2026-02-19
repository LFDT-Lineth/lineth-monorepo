import { fetchWithTimeout } from "@consensys/linea-shared-utils";

import { ISlackClient, SlackNotificationResult } from "../core/clients/ISlackClient.js";
import { Assessment, RiskLevel } from "../core/entities/Assessment.js";
import { ProposalWithoutText } from "../core/entities/Proposal.js";
import { ILidoGovernanceMonitorLogger } from "../utils/logging/index.js";

export class SlackClient implements ISlackClient {
  constructor(
    private readonly logger: ILidoGovernanceMonitorLogger,
    private readonly webhookUrl: string,
    private readonly riskThreshold: number,
    private readonly httpTimeoutMs: number,
    private readonly auditWebhookUrl?: string,
  ) {}

  async sendProposalAlert(proposal: ProposalWithoutText, assessment: Assessment): Promise<SlackNotificationResult> {
    const payload = this.buildAlertPayload(proposal, assessment);

    try {
      const response = await fetchWithTimeout(
        this.webhookUrl,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(payload),
        },
        this.httpTimeoutMs,
      );

      if (!response.ok) {
        const errorText = await response.text();
        this.logger.critical("Slack webhook failed", { status: response.status, error: errorText });
        return { success: false, error: errorText };
      }

      this.logger.info("Slack notification sent", { proposalId: proposal.id, title: proposal.title });
      return { success: true };
    } catch (error) {
      const errorMessage = error instanceof Error ? error.message : "Unknown error";
      this.logger.critical("Slack notification error", { error: errorMessage });
      return { success: false, error: errorMessage };
    }
  }

  async sendAuditLog(proposal: ProposalWithoutText, assessment: Assessment): Promise<SlackNotificationResult> {
    if (!this.auditWebhookUrl) {
      this.logger.debug("Audit webhook not configured, skipping");
      return { success: true };
    }

    const payload = this.buildAuditPayload(proposal, assessment);

    try {
      const response = await fetchWithTimeout(
        this.auditWebhookUrl,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(payload),
        },
        this.httpTimeoutMs,
      );

      if (!response.ok) {
        const errorText = await response.text();
        this.logger.critical("Audit webhook failed", { status: response.status, error: errorText });
        return { success: false, error: errorText };
      }

      this.logger.debug("Audit log sent", { proposalId: proposal.id });
      return { success: true };
    } catch (error) {
      const errorMessage = error instanceof Error ? error.message : "Unknown error";
      this.logger.critical("Audit log error", { error: errorMessage });
      return { success: false, error: errorMessage };
    }
  }

  private getRiskEmoji(riskLevel: RiskLevel): string {
    switch (riskLevel) {
      case "critical":
        return ":rotating_light:";
      case "high":
        return ":warning:";
      case "medium":
      case "low":
      default:
        return ":information_source:";
    }
  }

  private buildAlertPayload(proposal: ProposalWithoutText, assessment: Assessment): object {
    const riskEmoji = this.getRiskEmoji(assessment.riskLevel);

    return {
      blocks: [
        {
          type: "header",
          text: {
            type: "plain_text",
            text: this.truncateHeaderText(`${riskEmoji} Lido Governance Alert: ${proposal.title}`),
            emoji: true,
          },
        },
        ...this.buildSharedBlocks(proposal, assessment),
      ],
    };
  }

  private buildAuditPayload(proposal: ProposalWithoutText, assessment: Assessment): object {
    const wouldAlert = assessment.riskScore >= this.riskThreshold;

    return {
      blocks: [
        {
          type: "header",
          text: {
            type: "plain_text",
            text: this.truncateHeaderText(`📋 [AUDIT] ${proposal.title}`),
            emoji: true,
          },
        },
        {
          type: "context",
          elements: [
            {
              type: "mrkdwn",
              text: `*Assessment logged for manual review* • Risk: ${assessment.riskScore}/100 • ${wouldAlert ? "⚠️ Would trigger alert" : "ℹ️ Below alert threshold"}`,
            },
          ],
        },
        ...this.buildSharedBlocks(proposal, assessment),
      ],
    };
  }

  // Slack Block Kit header text has a 150-character limit.
  // Truncate with ellipsis to prevent webhook failures on long proposal titles.
  private truncateHeaderText(text: string): string {
    const SLACK_HEADER_MAX_LENGTH = 150;
    if (text.length <= SLACK_HEADER_MAX_LENGTH) return text;
    return text.slice(0, SLACK_HEADER_MAX_LENGTH - 1) + "…";
  }

  // Escapes characters that Slack mrkdwn interprets as links (<url|text>),
  // user mentions (<@U123>), and special mentions (<!here>).
  // `&` is escaped first to avoid double-escaping the other replacements.
  private escapeSlackMrkdwn(text: string): string {
    return text.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
  }

  // Builds the 7 Slack Block Kit blocks shared between alert and audit payloads.
  // Extracted to prevent formatting drift when either payload is updated.
  private buildSharedBlocks(proposal: ProposalWithoutText, assessment: Assessment): object[] {
    return [
      {
        type: "section",
        fields: [
          { type: "mrkdwn", text: `*Risk Score:* ${assessment.riskScore}/100` },
          { type: "mrkdwn", text: `*Risk Level:* ${assessment.riskLevel.toUpperCase()}` },
          { type: "mrkdwn", text: `*Confidence:* ${assessment.confidence}%` },
          { type: "mrkdwn", text: `*Urgency:* ${assessment.urgency.replace("_", " ")}` },
        ],
      },
      {
        type: "section",
        fields: [
          { type: "mrkdwn", text: `*Impact Types:* ${assessment.impactTypes.join(", ")}` },
          { type: "mrkdwn", text: `*Action:* ${assessment.recommendedAction}` },
        ],
      },
      {
        type: "section",
        text: {
          type: "mrkdwn",
          text: `*Affected Components:*\n${assessment.affectedComponents.join(", ")}`,
        },
      },
      {
        type: "section",
        text: {
          type: "mrkdwn",
          text: `*Invariants at Risk:*\n${assessment.nativeYieldInvariantsAtRisk.map((i) => `• ${i}`).join("\n")}`,
        },
      },
      {
        type: "section",
        text: { type: "mrkdwn", text: `*What Changed:*\n${this.escapeSlackMrkdwn(assessment.whatChanged)}` },
      },
      {
        type: "section",
        text: {
          type: "mrkdwn",
          text: `*What Is The Impact On Native Yield?*\n${assessment.nativeYieldImpact.map((i) => `- ${this.escapeSlackMrkdwn(i)}`).join("\n")}`,
        },
      },
      {
        type: "actions",
        elements: [
          {
            type: "button",
            text: { type: "plain_text", text: "View Proposal", emoji: true },
            url: proposal.url,
            action_id: "view_proposal",
          },
        ],
      },
    ];
  }
}
