import { jest, describe, it, expect, beforeEach, afterEach } from "@jest/globals";

import { ISlackClient } from "../../core/clients/ISlackClient.js";
import { Assessment, NativeYieldInvariant } from "../../core/entities/Assessment.js";
import { Proposal } from "../../core/entities/Proposal.js";
import { ProposalSource } from "../../core/entities/ProposalSource.js";
import { ProposalState } from "../../core/entities/ProposalState.js";
import { IProposalRepository } from "../../core/repositories/IProposalRepository.js";
import { ILidoGovernanceMonitorLogger } from "../../utils/logging/index.js";
import { NotificationService } from "../NotificationService.js";

const createLoggerMock = (): jest.Mocked<ILidoGovernanceMonitorLogger> => ({
  name: "test-logger",
  critical: jest.fn(),
  debug: jest.fn(),
  error: jest.fn(),
  info: jest.fn(),
  warn: jest.fn(),
});

describe("NotificationService", () => {
  let service: NotificationService;
  let logger: jest.Mocked<ILidoGovernanceMonitorLogger>;
  let slackClient: jest.Mocked<ISlackClient>;
  let proposalRepository: jest.Mocked<IProposalRepository>;

  const createMockAssessment = (): Assessment => ({
    riskScore: 75,
    riskLevel: "high",
    confidence: 85,
    proposalType: "discourse",
    impactTypes: ["technical"],
    affectedComponents: ["StakingVault"],
    whatChanged: "Contract upgrade",
    nativeYieldInvariantsAtRisk: [NativeYieldInvariant.VALID_YIELD_REPORTING],
    nativeYieldImpact: ["May affect withdrawals"],
    recommendedAction: "escalate",
    urgency: "urgent",
    supportingQuotes: ["quote"],
    keyUnknowns: [],
  });

  const createMockProposal = (overrides: Partial<Proposal> = {}): Proposal => ({
    id: "uuid-1",
    source: ProposalSource.DISCOURSE,
    sourceId: "12345",
    url: "https://research.lido.fi/t/test/12345",
    title: "Test Proposal",
    author: "testuser",
    sourceCreatedAt: new Date("2024-01-15"),
    rawProposalText: "Proposal content",
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
    assessmentJson: createMockAssessment(),
    riskScore: 75,
    notifyAttemptCount: 0,
    notifiedAt: null,
    ...overrides,
  });

  beforeEach(() => {
    logger = createLoggerMock();
    slackClient = {
      sendProposalAlert: jest.fn(),
      sendAuditLog: jest.fn(),
    } as jest.Mocked<ISlackClient>;
    proposalRepository = {
      findBySourceAndSourceId: jest.fn(),
      findByStateForAnalysis: jest.fn(),
      findByStateForNotification: jest.fn(),
      create: jest.fn(),
      upsert: jest.fn(),
      updateState: jest.fn(),
      attemptUpdateState: jest.fn(),
      saveAnalysis: jest.fn(),
      incrementAnalysisAttempt: jest.fn(),
      markNotified: jest.fn(),
      markNotifyFailed: jest.fn(),
      attemptMarkNotifyFailed: jest.fn(),
      findLatestSourceIdBySource: jest.fn(),
    } as jest.Mocked<IProposalRepository>;

    // Default audit log mock to return success
    slackClient.sendAuditLog.mockResolvedValue({ success: true });

    service = new NotificationService(
      logger,
      slackClient,
      proposalRepository,
      60, // riskThreshold
      5, // maxNotifyAttempts
    );
  });

  afterEach(() => {});

  describe("notifyOnce", () => {
    it("fetches ANALYZED and NOTIFY_FAILED proposals from repository with max attempts filter", async () => {
      // Arrange
      proposalRepository.findByStateForNotification.mockResolvedValue([]);

      // Act
      await service.notifyOnce();

      // Assert
      expect(proposalRepository.findByStateForNotification).toHaveBeenCalledWith(ProposalState.ANALYZED, 5);
      expect(proposalRepository.findByStateForNotification).toHaveBeenCalledWith(ProposalState.NOTIFY_FAILED, 5);
    });

    it("sends Slack notification for each pending proposal", async () => {
      // Arrange
      const proposal = createMockProposal();
      proposalRepository.findByStateForNotification
        .mockResolvedValueOnce([proposal]) // ANALYZED
        .mockResolvedValueOnce([]); // NOTIFY_FAILED
      slackClient.sendProposalAlert.mockResolvedValue({ success: true });
      proposalRepository.markNotified.mockResolvedValue(proposal);

      // Act
      await service.notifyOnce();

      // Assert
      expect(slackClient.sendProposalAlert).toHaveBeenCalledWith(proposal, proposal.assessmentJson);
    });

    it("marks proposal as NOTIFIED on successful Slack notification", async () => {
      // Arrange
      const proposal = createMockProposal();
      proposalRepository.findByStateForNotification
        .mockResolvedValueOnce([proposal]) // ANALYZED
        .mockResolvedValueOnce([]); // NOTIFY_FAILED
      slackClient.sendProposalAlert.mockResolvedValue({ success: true });
      proposalRepository.markNotified.mockResolvedValue({ ...proposal, state: ProposalState.NOTIFIED });

      // Act
      await service.notifyOnce();

      // Assert
      expect(proposalRepository.markNotified).toHaveBeenCalledWith(proposal.id);
      expect(logger.info).toHaveBeenCalledWith("Proposal notification sent", expect.any(Object));
    });

    it("marks as NOTIFY_FAILED and logs warning when Slack notification fails", async () => {
      // Arrange
      const proposal = createMockProposal({ notifyAttemptCount: 0 });
      proposalRepository.findByStateForNotification
        .mockResolvedValueOnce([proposal]) // ANALYZED
        .mockResolvedValueOnce([]); // NOTIFY_FAILED
      slackClient.sendProposalAlert.mockResolvedValue({ success: false, error: "Webhook failed" });
      proposalRepository.markNotifyFailed.mockResolvedValue({ ...proposal, notifyAttemptCount: 1 });

      // Act
      await service.notifyOnce();

      // Assert
      expect(proposalRepository.markNotifyFailed).toHaveBeenCalledWith(proposal.id);
      expect(logger.warn).toHaveBeenCalledWith("Slack notification failed, will retry", expect.any(Object));
    });

    it("retries NOTIFY_FAILED proposals", async () => {
      // Arrange
      const failedProposal = createMockProposal({
        state: ProposalState.NOTIFY_FAILED,
        notifyAttemptCount: 3,
      });
      proposalRepository.findByStateForNotification
        .mockResolvedValueOnce([]) // ANALYZED
        .mockResolvedValueOnce([failedProposal]); // NOTIFY_FAILED
      slackClient.sendProposalAlert.mockResolvedValue({ success: true });
      proposalRepository.markNotified.mockResolvedValue({ ...failedProposal, state: ProposalState.NOTIFIED });

      // Act
      await service.notifyOnce();

      // Assert
      expect(slackClient.sendProposalAlert).toHaveBeenCalled();
      expect(proposalRepository.markNotified).toHaveBeenCalledWith(failedProposal.id);
    });

    it("does nothing when no proposals need notification", async () => {
      // Arrange
      proposalRepository.findByStateForNotification.mockResolvedValue([]);

      // Act
      await service.notifyOnce();

      // Assert
      expect(slackClient.sendProposalAlert).not.toHaveBeenCalled();
      expect(logger.debug).toHaveBeenCalledWith("No proposals to notify");
    });

    it("skips proposals without assessment data and marks as NOTIFY_FAILED", async () => {
      // Arrange
      const proposalWithoutAssessment = createMockProposal({ assessmentJson: null });
      proposalRepository.findByStateForNotification
        .mockResolvedValueOnce([proposalWithoutAssessment]) // ANALYZED
        .mockResolvedValueOnce([]); // NOTIFY_FAILED
      proposalRepository.markNotifyFailed.mockResolvedValue({
        ...proposalWithoutAssessment,
        state: ProposalState.NOTIFY_FAILED,
        notifyAttemptCount: 1,
      });

      // Act
      await service.notifyOnce();

      // Assert
      expect(slackClient.sendProposalAlert).not.toHaveBeenCalled();
      expect(logger.error).toHaveBeenCalledWith("Proposal missing assessment data", expect.any(Object));
      expect(proposalRepository.markNotifyFailed).toHaveBeenCalledWith(proposalWithoutAssessment.id);
    });

    it("handles errors during notification gracefully and transitions to NOTIFY_FAILED", async () => {
      // Arrange
      const proposal = createMockProposal();
      proposalRepository.findByStateForNotification
        .mockResolvedValueOnce([proposal]) // ANALYZED
        .mockResolvedValueOnce([]); // NOTIFY_FAILED
      slackClient.sendProposalAlert.mockRejectedValue(new Error("Network error"));
      proposalRepository.attemptMarkNotifyFailed.mockResolvedValue(null);

      // Act
      await service.notifyOnce();

      // Assert
      expect(proposalRepository.attemptMarkNotifyFailed).toHaveBeenCalledWith(proposal.id);
      expect(logger.critical).toHaveBeenCalledWith("Error notifying proposal", expect.any(Object));
    });

    it("catches and logs errors without throwing", async () => {
      // Arrange
      proposalRepository.findByStateForNotification.mockRejectedValue(new Error("Database connection error"));

      // Act
      await service.notifyOnce();

      // Assert
      expect(logger.critical).toHaveBeenCalledWith("Notification processing failed", expect.any(Object));
    });

    it("skips notification for proposals below risk threshold", async () => {
      // Arrange
      const lowRiskProposal = createMockProposal({ riskScore: 30 }); // Below threshold of 60
      proposalRepository.findByStateForNotification
        .mockResolvedValueOnce([lowRiskProposal]) // ANALYZED
        .mockResolvedValueOnce([]); // NOTIFY_FAILED
      proposalRepository.updateState.mockResolvedValue({ ...lowRiskProposal, state: ProposalState.NOT_NOTIFIED });

      // Act
      await service.notifyOnce();

      // Assert
      expect(slackClient.sendProposalAlert).not.toHaveBeenCalled();
      expect(proposalRepository.updateState).toHaveBeenCalledWith(lowRiskProposal.id, ProposalState.NOT_NOTIFIED);
      expect(logger.info).toHaveBeenCalledWith("Proposal below notification threshold, skipped", expect.any(Object));
    });

    it("sends notification for proposals at or above risk threshold", async () => {
      // Arrange
      const highRiskProposal = createMockProposal({ riskScore: 60 }); // At threshold
      proposalRepository.findByStateForNotification
        .mockResolvedValueOnce([highRiskProposal]) // ANALYZED
        .mockResolvedValueOnce([]); // NOTIFY_FAILED
      slackClient.sendProposalAlert.mockResolvedValue({ success: true });
      proposalRepository.markNotified.mockResolvedValue({ ...highRiskProposal, state: ProposalState.NOTIFIED });

      // Act
      await service.notifyOnce();

      // Assert
      expect(slackClient.sendProposalAlert).toHaveBeenCalledWith(highRiskProposal, highRiskProposal.assessmentJson);
      expect(proposalRepository.markNotified).toHaveBeenCalledWith(highRiskProposal.id);
    });

    it("skips notification when riskScore is null", async () => {
      // Arrange
      const proposalWithoutRiskScore = createMockProposal({ riskScore: null });
      proposalRepository.findByStateForNotification
        .mockResolvedValueOnce([proposalWithoutRiskScore]) // ANALYZED
        .mockResolvedValueOnce([]); // NOTIFY_FAILED
      proposalRepository.updateState.mockResolvedValue({
        ...proposalWithoutRiskScore,
        state: ProposalState.NOT_NOTIFIED,
      });

      // Act
      await service.notifyOnce();

      // Assert
      expect(slackClient.sendProposalAlert).not.toHaveBeenCalled();
      expect(proposalRepository.updateState).toHaveBeenCalledWith(
        proposalWithoutRiskScore.id,
        ProposalState.NOT_NOTIFIED,
      );
    });

    it("sends audit log for high-risk proposals", async () => {
      // Arrange
      const highRiskProposal = createMockProposal({ riskScore: 75 });
      proposalRepository.findByStateForNotification.mockResolvedValueOnce([highRiskProposal]).mockResolvedValueOnce([]);
      slackClient.sendProposalAlert.mockResolvedValue({ success: true });
      proposalRepository.markNotified.mockResolvedValue({ ...highRiskProposal, state: ProposalState.NOTIFIED });

      // Act
      await service.notifyOnce();

      // Assert
      expect(slackClient.sendAuditLog).toHaveBeenCalledWith(highRiskProposal, highRiskProposal.assessmentJson);
    });

    it("sends audit log for low-risk proposals", async () => {
      // Arrange
      const lowRiskProposal = createMockProposal({ riskScore: 30 });
      proposalRepository.findByStateForNotification.mockResolvedValueOnce([lowRiskProposal]).mockResolvedValueOnce([]);
      proposalRepository.updateState.mockResolvedValue({ ...lowRiskProposal, state: ProposalState.NOT_NOTIFIED });

      // Act
      await service.notifyOnce();

      // Assert
      expect(slackClient.sendAuditLog).toHaveBeenCalledWith(lowRiskProposal, lowRiskProposal.assessmentJson);
    });

    it("continues with alert when audit log fails", async () => {
      // Arrange
      const highRiskProposal = createMockProposal({ riskScore: 75 });
      proposalRepository.findByStateForNotification.mockResolvedValueOnce([highRiskProposal]).mockResolvedValueOnce([]);
      slackClient.sendAuditLog.mockResolvedValue({ success: false, error: "Audit webhook failed" });
      slackClient.sendProposalAlert.mockResolvedValue({ success: true });
      proposalRepository.markNotified.mockResolvedValue({ ...highRiskProposal, state: ProposalState.NOTIFIED });

      // Act
      await service.notifyOnce();

      // Assert
      expect(logger.critical).toHaveBeenCalledWith("Audit log failed, continuing", expect.any(Object));
      expect(slackClient.sendProposalAlert).toHaveBeenCalledWith(highRiskProposal, highRiskProposal.assessmentJson);
      expect(proposalRepository.markNotified).toHaveBeenCalledWith(highRiskProposal.id);
    });

    it("marks low-risk proposal as NOT_NOTIFIED even if audit log fails", async () => {
      // Arrange
      const lowRiskProposal = createMockProposal({ riskScore: 30 });
      proposalRepository.findByStateForNotification.mockResolvedValueOnce([lowRiskProposal]).mockResolvedValueOnce([]);
      slackClient.sendAuditLog.mockResolvedValue({ success: false, error: "Audit webhook failed" });
      proposalRepository.updateState.mockResolvedValue({ ...lowRiskProposal, state: ProposalState.NOT_NOTIFIED });

      // Act
      await service.notifyOnce();

      // Assert
      expect(logger.critical).toHaveBeenCalledWith("Audit log failed, continuing", expect.any(Object));
      expect(proposalRepository.updateState).toHaveBeenCalledWith(lowRiskProposal.id, ProposalState.NOT_NOTIFIED);
    });

    it("skips notification when assessmentJson fails schema validation and marks as NOTIFY_FAILED", async () => {
      // Arrange - assessmentJson is missing required fields
      const malformedProposal = createMockProposal({
        assessmentJson: { riskScore: "not-a-number" } as unknown as Assessment,
      });
      proposalRepository.findByStateForNotification
        .mockResolvedValueOnce([malformedProposal])
        .mockResolvedValueOnce([]);
      proposalRepository.markNotifyFailed.mockResolvedValue({
        ...malformedProposal,
        state: ProposalState.NOTIFY_FAILED,
        notifyAttemptCount: 1,
      });

      // Act
      await service.notifyOnce();

      // Assert
      expect(slackClient.sendProposalAlert).not.toHaveBeenCalled();
      expect(slackClient.sendAuditLog).not.toHaveBeenCalled();
      expect(logger.error).toHaveBeenCalledWith(
        "Proposal assessmentJson failed schema validation",
        expect.objectContaining({ proposalId: malformedProposal.id }),
      );
      expect(proposalRepository.markNotifyFailed).toHaveBeenCalledWith(malformedProposal.id);
    });

    it("logs validation error details when assessmentJson is malformed", async () => {
      // Arrange - assessmentJson has wrong types for multiple fields
      const malformedProposal = createMockProposal({
        assessmentJson: {
          riskScore: 75,
          riskLevel: "invalid-level",
          confidence: 85,
          proposalType: "discourse",
          impactTypes: ["technical"],
          affectedComponents: ["StakingVault"],
          whatChanged: "Contract upgrade",
          nativeYieldInvariantsAtRisk: [],
          nativeYieldImpact: [],
          recommendedAction: "escalate",
          urgency: "urgent",
          supportingQuotes: [],
          keyUnknowns: [],
        } as unknown as Assessment,
      });
      proposalRepository.findByStateForNotification
        .mockResolvedValueOnce([malformedProposal])
        .mockResolvedValueOnce([]);
      proposalRepository.markNotifyFailed.mockResolvedValue({
        ...malformedProposal,
        state: ProposalState.NOTIFY_FAILED,
        notifyAttemptCount: 1,
      });

      // Act
      await service.notifyOnce();

      // Assert
      expect(logger.error).toHaveBeenCalledWith(
        "Proposal assessmentJson failed schema validation",
        expect.objectContaining({
          proposalId: malformedProposal.id,
          errors: expect.arrayContaining([expect.objectContaining({ path: expect.any(Array) })]),
        }),
      );
    });

    it("does not send audit log when proposal missing assessment", async () => {
      // Arrange
      const proposalWithoutAssessment = createMockProposal({ assessmentJson: null });
      proposalRepository.findByStateForNotification
        .mockResolvedValueOnce([proposalWithoutAssessment])
        .mockResolvedValueOnce([]);
      proposalRepository.markNotifyFailed.mockResolvedValue({
        ...proposalWithoutAssessment,
        state: ProposalState.NOTIFY_FAILED,
        notifyAttemptCount: 1,
      });

      // Act
      await service.notifyOnce();

      // Assert
      expect(slackClient.sendAuditLog).not.toHaveBeenCalled();
    });

    it("filters out proposals exceeding max notify attempts at query level", async () => {
      // Arrange - DB returns no proposals because they all exceeded max attempts
      proposalRepository.findByStateForNotification.mockResolvedValue([]);

      // Act
      await service.notifyOnce();

      // Assert - maxNotifyAttempts (5) is passed to both queries
      expect(proposalRepository.findByStateForNotification).toHaveBeenCalledWith(ProposalState.ANALYZED, 5);
      expect(proposalRepository.findByStateForNotification).toHaveBeenCalledWith(ProposalState.NOTIFY_FAILED, 5);
      expect(slackClient.sendProposalAlert).not.toHaveBeenCalled();
    });

    it("transitions proposal to NOTIFY_FAILED when Slack notification fails", async () => {
      // Arrange
      const proposal = createMockProposal({ riskScore: 75 }); // Above threshold
      proposalRepository.findByStateForNotification
        .mockResolvedValueOnce([proposal]) // ANALYZED
        .mockResolvedValueOnce([]); // NOTIFY_FAILED
      slackClient.sendAuditLog.mockResolvedValue({ success: true });
      slackClient.sendProposalAlert.mockResolvedValue({ success: false, error: "Webhook failed" });
      proposalRepository.markNotifyFailed.mockResolvedValue({
        ...proposal,
        state: ProposalState.NOTIFY_FAILED,
        notifyAttemptCount: 1,
      });

      // Act
      await service.notifyOnce();

      // Assert
      expect(proposalRepository.markNotifyFailed).toHaveBeenCalledWith(proposal.id);
      expect(logger.warn).toHaveBeenCalledWith("Slack notification failed, will retry", expect.any(Object));
      expect(proposalRepository.markNotified).not.toHaveBeenCalled();
    });
  });
});
