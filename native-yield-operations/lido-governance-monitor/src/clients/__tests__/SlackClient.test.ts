import { jest, describe, it, expect, beforeEach } from "@jest/globals";

import { Assessment, NativeYieldInvariant } from "../../core/entities/Assessment.js";
import { ProposalWithoutText } from "../../core/entities/Proposal.js";
import { ProposalSource } from "../../core/entities/ProposalSource.js";
import { ProposalState } from "../../core/entities/ProposalState.js";
import { ILidoGovernanceMonitorLogger } from "../../utils/logging/index.js";
import { SlackClient } from "../SlackClient.js";

const createLoggerMock = (): jest.Mocked<ILidoGovernanceMonitorLogger> => ({
  name: "test-logger",
  critical: jest.fn(),
  debug: jest.fn(),
  error: jest.fn(),
  info: jest.fn(),
  warn: jest.fn(),
});

describe("SlackClient", () => {
  let client: SlackClient;
  let logger: jest.Mocked<ILidoGovernanceMonitorLogger>;
  let fetchMock: jest.Mock;

  const createMockProposal = (overrides: Partial<ProposalWithoutText> = {}): ProposalWithoutText => ({
    id: "uuid-1",
    source: ProposalSource.DISCOURSE,
    sourceId: "12345",
    url: "https://research.lido.fi/t/test/12345",
    title: "Test Proposal",
    author: "testuser",
    sourceCreatedAt: new Date("2024-01-15"),
    sourceMetadata: null,
    state: ProposalState.ANALYZED,
    createdAt: new Date(),
    updatedAt: new Date(),
    stateUpdatedAt: new Date(),
    analysisAttemptCount: 1,
    llmModel: "claude-sonnet-4",
    riskThreshold: 60,
    assessmentPromptVersion: "v1.0",
    analyzedAt: new Date(),
    assessmentJson: null,
    riskScore: 75,
    notifyAttemptCount: 0,
    notifiedAt: null,
    ...overrides,
  });

  const createMockAssessment = (overrides: Partial<Assessment> = {}): Assessment => ({
    riskScore: 75,
    riskLevel: "high",
    confidence: 85,
    proposalType: "discourse",
    impactTypes: ["technical"],
    affectedComponents: ["StakingVault"],
    whatChanged: "Contract upgrade to StakingVault v2",
    nativeYieldInvariantsAtRisk: [NativeYieldInvariant.VALID_YIELD_REPORTING],
    nativeYieldImpact: ["May affect withdrawal mechanics and yield reporting"],
    recommendedAction: "escalate",
    urgency: "urgent",
    supportingQuotes: ["The upgrade will modify the withdrawal queue..."],
    keyUnknowns: [],
    ...overrides,
  });

  beforeEach(() => {
    logger = createLoggerMock();
    fetchMock = jest.fn();
    global.fetch = fetchMock as unknown as typeof fetch;
    client = new SlackClient(
      logger,
      "https://hooks.slack.com/services/xxx",
      60,
      15000,
      "https://hooks.slack.com/services/yyy",
    );
  });

  describe("sendProposalAlert", () => {
    it("sends formatted message to Slack webhook and returns success", async () => {
      // Arrange
      const mockProposal = createMockProposal();
      const mockAssessment = createMockAssessment();
      fetchMock.mockResolvedValue({ ok: true, text: () => Promise.resolve("ok") });

      // Act
      const result = await client.sendProposalAlert(mockProposal, mockAssessment);

      // Assert
      expect(result.success).toBe(true);
      expect(fetchMock).toHaveBeenCalledWith(
        "https://hooks.slack.com/services/xxx",
        expect.objectContaining({
          method: "POST",
          headers: { "Content-Type": "application/json" },
        }),
      );
    });

    it("includes risk level emoji based on severity", async () => {
      // Arrange
      const mockProposal = createMockProposal();
      const criticalAssessment = createMockAssessment({ riskLevel: "critical", riskScore: 90 });
      fetchMock.mockResolvedValue({ ok: true, text: () => Promise.resolve("ok") });

      // Act
      await client.sendProposalAlert(mockProposal, criticalAssessment);

      // Assert
      const callBody = JSON.parse(fetchMock.mock.calls[0][1].body);
      expect(callBody.blocks[0].text.text).toContain(":rotating_light:");
    });

    it("includes affected components and invariants in message", async () => {
      // Arrange
      const mockProposal = createMockProposal();
      const mockAssessment = createMockAssessment({
        affectedComponents: ["StakingVault", "VaultHub"],
        nativeYieldInvariantsAtRisk: [
          NativeYieldInvariant.VALID_YIELD_REPORTING,
          NativeYieldInvariant.USER_PRINCIPAL_PROTECTION,
        ],
      });
      fetchMock.mockResolvedValue({ ok: true, text: () => Promise.resolve("ok") });

      // Act
      await client.sendProposalAlert(mockProposal, mockAssessment);

      // Assert
      const callBody = JSON.parse(fetchMock.mock.calls[0][1].body);
      const bodyString = JSON.stringify(callBody);
      expect(bodyString).toContain("StakingVault");
      expect(bodyString).toContain("VaultHub");
    });

    it("returns failure on webhook error response", async () => {
      // Arrange
      const mockProposal = createMockProposal();
      const mockAssessment = createMockAssessment();
      fetchMock.mockResolvedValue({ ok: false, status: 500, text: () => Promise.resolve("internal_error") });

      // Act
      const result = await client.sendProposalAlert(mockProposal, mockAssessment);

      // Assert
      expect(result.success).toBe(false);
      expect(result.error).toBe("internal_error");
      expect(logger.critical).toHaveBeenCalled();
    });

    it("returns failure on network error", async () => {
      // Arrange
      const mockProposal = createMockProposal();
      const mockAssessment = createMockAssessment();
      fetchMock.mockRejectedValue(new Error("Network error"));

      // Act
      const result = await client.sendProposalAlert(mockProposal, mockAssessment);

      // Assert
      expect(result.success).toBe(false);
      expect(result.error).toBe("Network error");
      expect(logger.critical).toHaveBeenCalled();
    });

    it("returns failure with 'Unknown error' when non-Error object is thrown", async () => {
      // Arrange
      const mockProposal = createMockProposal();
      const mockAssessment = createMockAssessment();
      fetchMock.mockRejectedValue("string error"); // Non-Error object

      // Act
      const result = await client.sendProposalAlert(mockProposal, mockAssessment);

      // Assert
      expect(result.success).toBe(false);
      expect(result.error).toBe("Unknown error");
      expect(logger.critical).toHaveBeenCalledWith("Slack notification error", { error: "Unknown error" });
    });

    it("uses warning emoji for high risk level", async () => {
      // Arrange
      const mockProposal = createMockProposal();
      const highAssessment = createMockAssessment({ riskLevel: "high" });
      fetchMock.mockResolvedValue({ ok: true, text: () => Promise.resolve("ok") });

      // Act
      await client.sendProposalAlert(mockProposal, highAssessment);

      // Assert
      const callBody = JSON.parse(fetchMock.mock.calls[0][1].body);
      expect(callBody.blocks[0].text.text).toContain(":warning:");
    });

    it("uses info emoji for medium risk level", async () => {
      // Arrange
      const mockProposal = createMockProposal();
      const mediumAssessment = createMockAssessment({ riskLevel: "medium", riskScore: 50 });
      fetchMock.mockResolvedValue({ ok: true, text: () => Promise.resolve("ok") });

      // Act
      await client.sendProposalAlert(mockProposal, mediumAssessment);

      // Assert
      const callBody = JSON.parse(fetchMock.mock.calls[0][1].body);
      expect(callBody.blocks[0].text.text).toContain(":information_source:");
    });

    it("uses info emoji for low risk level", async () => {
      // Arrange
      const mockProposal = createMockProposal();
      const lowAssessment = createMockAssessment({ riskLevel: "low", riskScore: 25 });
      fetchMock.mockResolvedValue({ ok: true, text: () => Promise.resolve("ok") });

      // Act
      await client.sendProposalAlert(mockProposal, lowAssessment);

      // Assert
      const callBody = JSON.parse(fetchMock.mock.calls[0][1].body);
      expect(callBody.blocks[0].text.text).toContain(":information_source:");
    });
  });

  describe("sendAuditLog", () => {
    it("sends formatted audit message to audit webhook and returns success", async () => {
      // Arrange
      const mockProposal = createMockProposal();
      const mockAssessment = createMockAssessment();
      fetchMock.mockResolvedValue({ ok: true, text: () => Promise.resolve("ok") });

      // Act
      const result = await client.sendAuditLog(mockProposal, mockAssessment);

      // Assert
      expect(result.success).toBe(true);
      expect(fetchMock).toHaveBeenCalledWith(
        "https://hooks.slack.com/services/yyy",
        expect.objectContaining({
          method: "POST",
          headers: { "Content-Type": "application/json" },
        }),
      );
    });

    it("includes [AUDIT] prefix in message title", async () => {
      // Arrange
      const mockProposal = createMockProposal();
      const mockAssessment = createMockAssessment();
      fetchMock.mockResolvedValue({ ok: true, text: () => Promise.resolve("ok") });

      // Act
      await client.sendAuditLog(mockProposal, mockAssessment);

      // Assert
      const callBody = JSON.parse(fetchMock.mock.calls[0][1].body);
      expect(callBody.blocks[0].text.text).toContain("📋 [AUDIT]");
    });

    it("includes threshold context for high-risk proposals", async () => {
      // Arrange
      const mockProposal = createMockProposal({ riskScore: 75 });
      const mockAssessment = createMockAssessment({ riskScore: 75 });
      fetchMock.mockResolvedValue({ ok: true, text: () => Promise.resolve("ok") });

      // Act
      await client.sendAuditLog(mockProposal, mockAssessment);

      // Assert
      const callBody = JSON.parse(fetchMock.mock.calls[0][1].body);
      const bodyString = JSON.stringify(callBody);
      expect(bodyString).toContain("Would trigger alert");
    });

    it("includes threshold context for low-risk proposals", async () => {
      // Arrange
      const mockProposal = createMockProposal({ riskScore: 30 });
      const mockAssessment = createMockAssessment({ riskScore: 30 });
      fetchMock.mockResolvedValue({ ok: true, text: () => Promise.resolve("ok") });

      // Act
      await client.sendAuditLog(mockProposal, mockAssessment);

      // Assert
      const callBody = JSON.parse(fetchMock.mock.calls[0][1].body);
      const bodyString = JSON.stringify(callBody);
      expect(bodyString).toContain("Below alert threshold");
    });

    it("returns success when audit webhook not configured", async () => {
      // Arrange
      const clientWithoutAudit = new SlackClient(logger, "https://hooks.slack.com/services/xxx", 60, 15000);
      const mockProposal = createMockProposal();
      const mockAssessment = createMockAssessment();

      // Act
      const result = await clientWithoutAudit.sendAuditLog(mockProposal, mockAssessment);

      // Assert
      expect(result.success).toBe(true);
      expect(fetchMock).not.toHaveBeenCalled();
      expect(logger.debug).toHaveBeenCalledWith("Audit webhook not configured, skipping");
    });

    it("returns failure on audit webhook error response", async () => {
      // Arrange
      const mockProposal = createMockProposal();
      const mockAssessment = createMockAssessment();
      fetchMock.mockResolvedValue({ ok: false, status: 500, text: () => Promise.resolve("internal_error") });

      // Act
      const result = await client.sendAuditLog(mockProposal, mockAssessment);

      // Assert
      expect(result.success).toBe(false);
      expect(result.error).toBe("internal_error");
      expect(logger.critical).toHaveBeenCalled();
    });

    it("returns failure on audit network error", async () => {
      // Arrange
      const mockProposal = createMockProposal();
      const mockAssessment = createMockAssessment();
      fetchMock.mockRejectedValue(new Error("Network error"));

      // Act
      const result = await client.sendAuditLog(mockProposal, mockAssessment);

      // Assert
      expect(result.success).toBe(false);
      expect(result.error).toBe("Network error");
      expect(logger.critical).toHaveBeenCalled();
    });

    it("returns failure with 'Unknown error' when non-Error object is thrown", async () => {
      // Arrange
      const mockProposal = createMockProposal();
      const mockAssessment = createMockAssessment();
      fetchMock.mockRejectedValue({ code: "FETCH_ERROR" }); // Non-Error object

      // Act
      const result = await client.sendAuditLog(mockProposal, mockAssessment);

      // Assert
      expect(result.success).toBe(false);
      expect(result.error).toBe("Unknown error");
      expect(logger.critical).toHaveBeenCalledWith("Audit log error", { error: "Unknown error" });
    });

    it("uses configured threshold for audit message context", async () => {
      // Arrange - threshold is 75
      const clientWithCustomThreshold = new SlackClient(
        logger,
        "https://hooks.slack.com/services/xxx",
        75,
        15000,
        "https://hooks.slack.com/services/yyy",
      );
      const proposalBelowThreshold = createMockProposal({ riskScore: 70 });
      const assessmentBelowThreshold = createMockAssessment({ riskScore: 70 });
      fetchMock.mockResolvedValue({ ok: true, text: () => Promise.resolve("ok") });

      // Act
      await clientWithCustomThreshold.sendAuditLog(proposalBelowThreshold, assessmentBelowThreshold);

      // Assert
      const callBody = JSON.parse(fetchMock.mock.calls[0][1].body);
      const bodyString = JSON.stringify(callBody);
      expect(bodyString).toContain("Below alert threshold");
      expect(bodyString).not.toContain("Would trigger alert");
    });

    it("shows 'Would trigger alert' when score equals threshold", async () => {
      // Arrange - threshold is 70
      const clientWithCustomThreshold = new SlackClient(
        logger,
        "https://hooks.slack.com/services/xxx",
        70,
        15000,
        "https://hooks.slack.com/services/yyy",
      );
      const proposalAtThreshold = createMockProposal({ riskScore: 70 });
      const assessmentAtThreshold = createMockAssessment({ riskScore: 70 });
      fetchMock.mockResolvedValue({ ok: true, text: () => Promise.resolve("ok") });

      // Act
      await clientWithCustomThreshold.sendAuditLog(proposalAtThreshold, assessmentAtThreshold);

      // Assert
      const callBody = JSON.parse(fetchMock.mock.calls[0][1].body);
      const bodyString = JSON.stringify(callBody);
      expect(bodyString).toContain("Would trigger alert");
    });
  });

  describe("shared block consistency", () => {
    it("alert and audit payloads contain identical content blocks after their unique headers", async () => {
      // Arrange
      const mockProposal = createMockProposal();
      const mockAssessment = createMockAssessment();
      fetchMock.mockResolvedValue({ ok: true, text: () => Promise.resolve("ok") });

      // Act - send both payloads
      await client.sendProposalAlert(mockProposal, mockAssessment);
      const alertBody = JSON.parse(fetchMock.mock.calls[0][1].body);

      await client.sendAuditLog(mockProposal, mockAssessment);
      const auditBody = JSON.parse(fetchMock.mock.calls[1][1].body);

      // Assert - alert has 1 unique header block, audit has 2 (header + context)
      // The shared blocks start at index 1 for alert and index 2 for audit
      const alertSharedBlocks = alertBody.blocks.slice(1);
      const auditSharedBlocks = auditBody.blocks.slice(2);

      expect(alertSharedBlocks).toEqual(auditSharedBlocks);
    });
  });

  describe("header text truncation", () => {
    it("truncates alert header to 150 characters when proposal title is long", async () => {
      // Arrange
      const longTitle = "A".repeat(200);
      const mockProposal = createMockProposal({ title: longTitle });
      const mockAssessment = createMockAssessment();
      fetchMock.mockResolvedValue({ ok: true, text: () => Promise.resolve("ok") });

      // Act
      await client.sendProposalAlert(mockProposal, mockAssessment);

      // Assert
      const callBody = JSON.parse(fetchMock.mock.calls[0][1].body);
      const headerText = callBody.blocks[0].text.text;
      expect(headerText.length).toBe(150);
      expect(headerText.endsWith("…")).toBe(true);
    });

    it("truncates audit header to 150 characters when proposal title is long", async () => {
      // Arrange
      const longTitle = "B".repeat(200);
      const mockProposal = createMockProposal({ title: longTitle });
      const mockAssessment = createMockAssessment();
      fetchMock.mockResolvedValue({ ok: true, text: () => Promise.resolve("ok") });

      // Act
      await client.sendAuditLog(mockProposal, mockAssessment);

      // Assert
      const callBody = JSON.parse(fetchMock.mock.calls[0][1].body);
      const headerText = callBody.blocks[0].text.text;
      expect(headerText.length).toBe(150);
      expect(headerText.endsWith("…")).toBe(true);
    });

    it("does not truncate alert header when title is short enough", async () => {
      // Arrange
      const mockProposal = createMockProposal({ title: "Short Title" });
      const mockAssessment = createMockAssessment();
      fetchMock.mockResolvedValue({ ok: true, text: () => Promise.resolve("ok") });

      // Act
      await client.sendProposalAlert(mockProposal, mockAssessment);

      // Assert
      const callBody = JSON.parse(fetchMock.mock.calls[0][1].body);
      const headerText = callBody.blocks[0].text.text;
      expect(headerText).toBe(":warning: Lido Governance Alert: Short Title");
      expect(headerText).not.toContain("…");
    });
  });

  describe("mrkdwn escaping in free-text assessment fields", () => {
    it("escapes angle brackets and ampersands in whatChanged", async () => {
      // Arrange
      const mockProposal = createMockProposal();
      const mockAssessment = createMockAssessment({
        whatChanged: "See <https://evil.com|details> & <!here> for more",
      });
      fetchMock.mockResolvedValue({ ok: true, text: () => Promise.resolve("ok") });

      // Act
      await client.sendProposalAlert(mockProposal, mockAssessment);

      // Assert
      const callBody = JSON.parse(fetchMock.mock.calls[0][1].body);
      const whatChangedBlock = callBody.blocks.find((b: { text?: { text?: string } }) =>
        b.text?.text?.includes("What Changed"),
      );
      expect(whatChangedBlock.text.text).toContain("&lt;https://evil.com|details&gt;");
      expect(whatChangedBlock.text.text).toContain("&amp;");
      expect(whatChangedBlock.text.text).toContain("&lt;!here&gt;");
      expect(whatChangedBlock.text.text).not.toContain("<https://evil.com|details>");
    });

    it("escapes angle brackets in nativeYieldImpact entries", async () => {
      // Arrange
      const mockProposal = createMockProposal();
      const mockAssessment = createMockAssessment({
        nativeYieldImpact: ["Impact <@U12345> mention", "Normal impact text"],
      });
      fetchMock.mockResolvedValue({ ok: true, text: () => Promise.resolve("ok") });

      // Act
      await client.sendProposalAlert(mockProposal, mockAssessment);

      // Assert
      const callBody = JSON.parse(fetchMock.mock.calls[0][1].body);
      const impactBlock = callBody.blocks.find((b: { text?: { text?: string } }) =>
        b.text?.text?.includes("Impact On Native Yield"),
      );
      expect(impactBlock.text.text).toContain("&lt;@U12345&gt;");
      expect(impactBlock.text.text).not.toContain("<@U12345>");
      expect(impactBlock.text.text).toContain("Normal impact text");
    });
  });

  describe("critical logging for notification failures", () => {
    it("logs critical error when both alert and audit channels fail", async () => {
      // Arrange
      const mockProposal = createMockProposal();
      const mockAssessment = createMockAssessment();
      fetchMock.mockResolvedValue({ ok: false, status: 500, text: () => Promise.resolve("service_unavailable") });

      // Act - call both notification methods
      await client.sendProposalAlert(mockProposal, mockAssessment);
      await client.sendAuditLog(mockProposal, mockAssessment);

      // Assert - verify logger.critical was called for BOTH failures
      expect(logger.critical).toHaveBeenCalledTimes(2);
      expect(logger.critical).toHaveBeenCalledWith("Slack webhook failed", expect.objectContaining({ status: 500 }));
      expect(logger.critical).toHaveBeenCalledWith("Audit webhook failed", expect.objectContaining({ status: 500 }));
    });
  });
});
