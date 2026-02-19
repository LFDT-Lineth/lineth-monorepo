import { ILogger } from "@consensys/linea-shared-utils";
import { jest, describe, it, expect, beforeEach } from "@jest/globals";

import { IAIClient } from "../../core/clients/IAIClient.js";
import { ISlackClient } from "../../core/clients/ISlackClient.js";
import { Assessment, NativeYieldInvariant } from "../../core/entities/Assessment.js";
import { Proposal } from "../../core/entities/Proposal.js";
import { ProposalSource } from "../../core/entities/ProposalSource.js";
import { ProposalState } from "../../core/entities/ProposalState.js";
import { IProposalRepository } from "../../core/repositories/IProposalRepository.js";
import { NotificationService } from "../../services/NotificationService.js";
import { ProposalProcessor } from "../../services/ProposalProcessor.js";

const createLoggerMock = (): jest.Mocked<ILogger> => ({
  name: "test-logger",
  debug: jest.fn(),
  error: jest.fn(),
  info: jest.fn(),
  warn: jest.fn(),
});

describe("Proposal Lifecycle Integration", () => {
  let logger: jest.Mocked<ILogger>;
  let aiClient: jest.Mocked<IAIClient>;
  let slackClient: jest.Mocked<ISlackClient>;
  let proposalRepository: jest.Mocked<IProposalRepository>;
  let processor: ProposalProcessor;
  let notificationService: NotificationService;

  const createMockAssessment = (riskScore: number): Assessment => ({
    riskScore,
    riskLevel: riskScore >= 61 ? "high" : riskScore >= 31 ? "medium" : "low",
    confidence: 85,
    proposalType: "discourse",
    impactTypes: ["technical"],
    affectedComponents: ["StakingVault"],
    whatChanged: "Contract upgrade",
    nativeYieldInvariantsAtRisk: riskScore >= 61 ? [NativeYieldInvariant.VALID_YIELD_REPORTING] : [],
    nativeYieldImpact: ["May affect withdrawals"],
    recommendedAction: riskScore >= 71 ? "escalate" : riskScore >= 51 ? "comment" : "monitor",
    urgency: riskScore >= 71 ? "urgent" : "none",
    supportingQuotes: ["quote"],
    keyUnknowns: [],
  });

  const createMockProposal = (state: ProposalState, assessment?: Assessment): Proposal => ({
    id: "uuid-1",
    source: ProposalSource.DISCOURSE,
    sourceId: "12345",
    url: "https://research.lido.fi/t/test/12345",
    title: "Test Proposal",
    author: "testuser",
    sourceCreatedAt: new Date("2024-01-15"),
    rawProposalText: "Proposal content",
    sourceMetadata: null,
    state,
    createdAt: new Date(),
    updatedAt: new Date(),
    stateUpdatedAt: new Date(),
    analysisAttemptCount: 0,
    llmModel: assessment ? "claude-sonnet-4" : null,
    riskThreshold: assessment ? 60 : null,
    assessmentPromptVersion: assessment ? "v1.0" : null,
    analyzedAt: assessment ? new Date() : null,
    assessmentJson: assessment ?? null,
    riskScore: assessment?.riskScore ?? null,
    notifyAttemptCount: 0,
    notifiedAt: null,
  });

  beforeEach(() => {
    logger = createLoggerMock();
    aiClient = {
      analyzeProposal: jest.fn(),
      getModelName: jest.fn().mockReturnValue("claude-sonnet-4"),
    } as jest.Mocked<IAIClient>;
    slackClient = {
      sendProposalAlert: jest.fn(),
      sendAuditLog: jest.fn().mockResolvedValue({ success: true }),
    } as jest.Mocked<ISlackClient>;
    proposalRepository = {
      findBySourceAndSourceId: jest.fn(),
      findByStateForAnalysis: jest.fn(),
      findByStateForNotification: jest.fn(),
      create: jest.fn(),
      updateState: jest.fn(),
      saveAnalysis: jest.fn(),
      incrementAnalysisAttempt: jest.fn(),
      incrementNotifyAttempt: jest.fn(),
      markNotified: jest.fn(),
    } as jest.Mocked<IProposalRepository>;

    processor = new ProposalProcessor(
      logger,
      aiClient,
      proposalRepository,
      60, // riskThreshold
      "v1.0",
      5, // maxAnalysisAttempts
    );

    notificationService = new NotificationService(logger, slackClient, proposalRepository, 60, 5);
  });

  describe("Full lifecycle: NEW → ANALYZED → NOTIFIED", () => {
    it("processes high-risk proposal through complete lifecycle", async () => {
      // Arrange
      const highRiskAssessment = createMockAssessment(75);
      const newProposal = createMockProposal(ProposalState.NEW);
      const analyzedProposal = createMockProposal(ProposalState.ANALYZED, highRiskAssessment);
      analyzedProposal.riskScore = 75; // Ensure riskScore is set
      const notifiedProposal = createMockProposal(ProposalState.NOTIFIED, highRiskAssessment);

      // Phase 1: Process NEW proposal
      proposalRepository.findByStateForAnalysis
        .mockResolvedValueOnce([newProposal]) // NEW
        .mockResolvedValueOnce([]); // ANALYSIS_FAILED
      proposalRepository.incrementAnalysisAttempt.mockResolvedValue({ ...newProposal, analysisAttemptCount: 1 });
      aiClient.analyzeProposal.mockResolvedValue(highRiskAssessment);
      proposalRepository.saveAnalysis.mockResolvedValue(analyzedProposal);

      // Act - Phase 1
      await processor.processOnce();

      // Assert - Phase 1: Should end in ANALYZED, not PENDING_NOTIFY
      expect(aiClient.analyzeProposal).toHaveBeenCalled();
      expect(proposalRepository.saveAnalysis).toHaveBeenCalledWith(
        newProposal.id,
        highRiskAssessment,
        75,
        "claude-sonnet-4",
        60,
        "v1.0",
      );
      expect(proposalRepository.updateState).not.toHaveBeenCalled(); // No longer called by processor

      // Phase 2: Notify ANALYZED proposal (not PENDING_NOTIFY)
      proposalRepository.findByStateForNotification
        .mockResolvedValueOnce([analyzedProposal]) // ANALYZED
        .mockResolvedValueOnce([]); // NOTIFY_FAILED
      proposalRepository.incrementNotifyAttempt.mockResolvedValue({ ...analyzedProposal, notifyAttemptCount: 1 });
      slackClient.sendProposalAlert.mockResolvedValue({ success: true });
      proposalRepository.markNotified.mockResolvedValue(notifiedProposal);

      // Act - Phase 2
      await notificationService.notifyOnce();

      // Assert - Phase 2
      expect(slackClient.sendProposalAlert).toHaveBeenCalledWith(analyzedProposal, highRiskAssessment);
      expect(proposalRepository.markNotified).toHaveBeenCalledWith(analyzedProposal.id);
    });
  });

  describe("Low-risk proposal lifecycle: NEW → ANALYZED → NOT_NOTIFIED", () => {
    it("processes low-risk proposal and skips notification", async () => {
      // Arrange
      const lowRiskAssessment = createMockAssessment(30);
      const newProposal = createMockProposal(ProposalState.NEW);
      const analyzedProposal = createMockProposal(ProposalState.ANALYZED, lowRiskAssessment);
      analyzedProposal.riskScore = 30;
      const notNotifiedProposal = { ...analyzedProposal, state: ProposalState.NOT_NOTIFIED };

      proposalRepository.findByStateForAnalysis
        .mockResolvedValueOnce([newProposal]) // NEW
        .mockResolvedValueOnce([]); // ANALYSIS_FAILED
      proposalRepository.incrementAnalysisAttempt.mockResolvedValue({ ...newProposal, analysisAttemptCount: 1 });
      aiClient.analyzeProposal.mockResolvedValue(lowRiskAssessment);
      proposalRepository.saveAnalysis.mockResolvedValue(analyzedProposal);

      // Act
      await processor.processOnce();

      // Assert - ProposalProcessor should only analyze, not transition to NOT_NOTIFIED
      expect(proposalRepository.saveAnalysis).toHaveBeenCalled();
      expect(proposalRepository.updateState).not.toHaveBeenCalled();

      // NotificationService will handle the NOT_NOTIFIED transition
      proposalRepository.findByStateForNotification.mockResolvedValueOnce([analyzedProposal]).mockResolvedValueOnce([]);
      proposalRepository.updateState.mockResolvedValue(notNotifiedProposal);

      await notificationService.notifyOnce();

      expect(proposalRepository.updateState).toHaveBeenCalledWith(analyzedProposal.id, ProposalState.NOT_NOTIFIED);
      expect(slackClient.sendProposalAlert).not.toHaveBeenCalled();
    });
  });

  describe("Analysis retry: ANALYSIS_FAILED → ANALYZED", () => {
    it("transitions to ANALYSIS_FAILED on AI failure, then retries successfully", async () => {
      // Arrange - First attempt fails
      const newProposal = createMockProposal(ProposalState.NEW);
      const failedProposal = { ...newProposal, state: ProposalState.ANALYSIS_FAILED, analysisAttemptCount: 1 };
      const highRiskAssessment = createMockAssessment(75);
      const analyzedProposal = createMockProposal(ProposalState.ANALYZED, highRiskAssessment);

      // First cycle: AI fails
      proposalRepository.findByStateForAnalysis
        .mockResolvedValueOnce([newProposal]) // NEW
        .mockResolvedValueOnce([]); // ANALYSIS_FAILED
      proposalRepository.incrementAnalysisAttempt.mockResolvedValue({ ...newProposal, analysisAttemptCount: 1 });
      aiClient.analyzeProposal.mockResolvedValueOnce(undefined); // Fail
      proposalRepository.updateState.mockResolvedValue(failedProposal);

      // Act - First cycle
      await processor.processOnce();

      // Assert - Should transition to ANALYSIS_FAILED
      expect(proposalRepository.updateState).toHaveBeenCalledWith(newProposal.id, ProposalState.ANALYSIS_FAILED);

      // Second cycle: AI succeeds
      proposalRepository.findByStateForAnalysis
        .mockResolvedValueOnce([]) // NEW
        .mockResolvedValueOnce([failedProposal]); // ANALYSIS_FAILED
      proposalRepository.incrementAnalysisAttempt.mockResolvedValue({ ...failedProposal, analysisAttemptCount: 2 });
      aiClient.analyzeProposal.mockResolvedValueOnce(highRiskAssessment); // Success
      proposalRepository.saveAnalysis.mockResolvedValue(analyzedProposal);

      // Act - Second cycle
      await processor.processOnce();

      // Assert - Should save analysis
      expect(proposalRepository.saveAnalysis).toHaveBeenCalledWith(
        failedProposal.id,
        highRiskAssessment,
        75,
        "claude-sonnet-4",
        60,
        "v1.0",
      );
    });

    it("retries ANALYSIS_FAILED proposals and succeeds", async () => {
      // Arrange
      const highRiskAssessment = createMockAssessment(75);
      const failedProposal = createMockProposal(ProposalState.ANALYSIS_FAILED);
      failedProposal.analysisAttemptCount = 3; // Previous failed attempts

      // First call returns no NEW, second returns the failed proposal
      proposalRepository.findByStateForAnalysis
        .mockResolvedValueOnce([]) // NEW
        .mockResolvedValueOnce([failedProposal]); // ANALYSIS_FAILED
      aiClient.analyzeProposal.mockResolvedValue(highRiskAssessment);
      proposalRepository.incrementAnalysisAttempt.mockResolvedValue({ ...failedProposal, analysisAttemptCount: 4 });
      proposalRepository.saveAnalysis.mockResolvedValue({ ...failedProposal, state: ProposalState.ANALYZED });

      // Act
      await processor.processOnce();

      // Assert
      expect(aiClient.analyzeProposal).toHaveBeenCalled();
      expect(proposalRepository.saveAnalysis).toHaveBeenCalled();
      expect(proposalRepository.updateState).not.toHaveBeenCalled();
    });
  });

  describe("Notification retry: NOTIFY_FAILED → NOTIFIED", () => {
    it("transitions to NOTIFY_FAILED on Slack failure, then retries successfully", async () => {
      // Arrange - First attempt fails
      const assessment = createMockAssessment(75);
      const analyzedProposal = createMockProposal(ProposalState.ANALYZED, assessment);
      const failedProposal = { ...analyzedProposal, state: ProposalState.NOTIFY_FAILED, notifyAttemptCount: 1 };
      const notifiedProposal = { ...analyzedProposal, state: ProposalState.NOTIFIED };

      // First cycle: Slack fails
      proposalRepository.findByStateForNotification
        .mockResolvedValueOnce([analyzedProposal]) // ANALYZED
        .mockResolvedValueOnce([]); // NOTIFY_FAILED
      slackClient.sendAuditLog.mockResolvedValue({ success: true });
      proposalRepository.incrementNotifyAttempt.mockResolvedValue({ ...analyzedProposal, notifyAttemptCount: 1 });
      slackClient.sendProposalAlert.mockResolvedValue({ success: false, error: "Webhook failed" });
      proposalRepository.updateState.mockResolvedValue(failedProposal);

      // Act - First cycle
      await notificationService.notifyOnce();

      // Assert - Should transition to NOTIFY_FAILED
      expect(proposalRepository.updateState).toHaveBeenCalledWith(analyzedProposal.id, ProposalState.NOTIFY_FAILED);

      // Second cycle: Slack succeeds
      proposalRepository.findByStateForNotification
        .mockResolvedValueOnce([]) // ANALYZED
        .mockResolvedValueOnce([failedProposal]); // NOTIFY_FAILED
      slackClient.sendAuditLog.mockResolvedValue({ success: true });
      proposalRepository.incrementNotifyAttempt.mockResolvedValue({ ...failedProposal, notifyAttemptCount: 2 });
      slackClient.sendProposalAlert.mockResolvedValue({ success: true });
      proposalRepository.markNotified.mockResolvedValue(notifiedProposal);

      // Act - Second cycle
      await notificationService.notifyOnce();

      // Assert - Should mark as notified
      expect(proposalRepository.markNotified).toHaveBeenCalledWith(failedProposal.id);
    });

    it("retries NOTIFY_FAILED proposals and succeeds", async () => {
      // Arrange
      const assessment = createMockAssessment(75);
      const failedProposal = createMockProposal(ProposalState.NOTIFY_FAILED, assessment);
      failedProposal.notifyAttemptCount = 3; // Previous failed attempts

      // First call returns no ANALYZED, second returns the failed proposal
      proposalRepository.findByStateForNotification
        .mockResolvedValueOnce([]) // ANALYZED
        .mockResolvedValueOnce([failedProposal]); // NOTIFY_FAILED
      slackClient.sendProposalAlert.mockResolvedValue({ success: true });
      proposalRepository.incrementNotifyAttempt.mockResolvedValue({ ...failedProposal, notifyAttemptCount: 4 });
      proposalRepository.markNotified.mockResolvedValue({ ...failedProposal, state: ProposalState.NOTIFIED });

      // Act
      await notificationService.notifyOnce();

      // Assert
      expect(slackClient.sendProposalAlert).toHaveBeenCalled();
      expect(proposalRepository.markNotified).toHaveBeenCalledWith(failedProposal.id);
    });
  });
});
