import Anthropic from "@anthropic-ai/sdk";
import { ExponentialBackoffRetryService } from "@consensys/linea-shared-utils";
import { PrismaPg } from "@prisma/adapter-pg";
import { createPublicClient, http } from "viem";
import { mainnet } from "viem/chains";

import { Config } from "./config/index.js";
import { PrismaClient } from "../../../prisma/client/client.js";
import { ClaudeAIClient } from "../../clients/ClaudeAIClient.js";
import { ProposalRepository } from "../../clients/db/ProposalRepository.js";
import { DiscourseClient } from "../../clients/DiscourseClient.js";
import { SlackClient } from "../../clients/SlackClient.js";
import { DiscourseFetcher } from "../../services/fetchers/DiscourseFetcher.js";
import { LdoVotingContractFetcher } from "../../services/fetchers/LdoVotingContractFetcher.js";
import { NormalizationService } from "../../services/NormalizationService.js";
import { NotificationService } from "../../services/NotificationService.js";
import { ProposalFetcher } from "../../services/ProposalFetcher.js";
import { ProposalProcessor } from "../../services/ProposalProcessor.js";
import { createLidoGovernanceMonitorLogger, ILidoGovernanceMonitorLogger } from "../../utils/logging/index.js";

export class LidoGovernanceMonitorBootstrap {
  private constructor(
    private readonly logger: ILidoGovernanceMonitorLogger,
    private readonly prisma: PrismaClient,
    private readonly proposalFetcher: ProposalFetcher,
    private readonly proposalProcessor: ProposalProcessor,
    private readonly notificationService: NotificationService,
  ) {}

  static create(config: Config, systemPrompt: string): LidoGovernanceMonitorBootstrap {
    const bootstrapLogger = createLidoGovernanceMonitorLogger("LidoGovernanceMonitorBootstrap");

    // Database
    const adapter = new PrismaPg({
      connectionString: config.database.url,
    });
    const prisma = new PrismaClient({ adapter });

    // Repositories
    const proposalRepository = new ProposalRepository(createLidoGovernanceMonitorLogger("ProposalRepository"), prisma);

    // Shared services
    const retryService = new ExponentialBackoffRetryService(createLidoGovernanceMonitorLogger("RetryService"));

    // Clients
    const discourseClient = new DiscourseClient(
      createLidoGovernanceMonitorLogger("DiscourseClient"),
      retryService,
      config.discourse.proposalsUrl,
      config.http.timeoutMs,
    );

    const anthropicClient = new Anthropic({ apiKey: config.anthropic.apiKey });
    const aiClient = new ClaudeAIClient(
      createLidoGovernanceMonitorLogger("ClaudeAIClient"),
      anthropicClient,
      config.anthropic.model,
      systemPrompt,
      config.anthropic.maxOutputTokens,
      config.anthropic.maxProposalChars,
    );

    const slackClient = new SlackClient(
      createLidoGovernanceMonitorLogger("SlackClient"),
      config.slack.webhookUrl,
      config.riskAssessment.threshold,
      config.http.timeoutMs,
      config.slack.auditWebhookUrl,
    );

    // Services
    const normalizationService = new NormalizationService(
      createLidoGovernanceMonitorLogger("NormalizationService"),
      discourseClient.getBaseUrl(),
    );

    // Source fetchers
    const discourseFetcher = new DiscourseFetcher(
      createLidoGovernanceMonitorLogger("DiscourseFetcher"),
      discourseClient,
      normalizationService,
      proposalRepository,
      config.discourse.maxTopicsPerPoll,
    );

    const ethereumClient = createPublicClient({
      chain: mainnet,
      transport: http(config.ethereum.rpcUrl),
    });

    const ldoVotingContractFetcher = new LdoVotingContractFetcher(
      createLidoGovernanceMonitorLogger("LdoVotingContractFetcher"),
      ethereumClient,
      config.ethereum.ldoVotingContractAddress,
      config.ethereum.initialLdoVotingContractVoteId != null
        ? BigInt(config.ethereum.initialLdoVotingContractVoteId)
        : undefined,
      proposalRepository,
    );

    const proposalFetcher = new ProposalFetcher(createLidoGovernanceMonitorLogger("ProposalFetcher"), [
      discourseFetcher,
      ldoVotingContractFetcher,
    ]);

    const proposalProcessor = new ProposalProcessor(
      createLidoGovernanceMonitorLogger("ProposalProcessor"),
      aiClient,
      proposalRepository,
      config.riskAssessment.threshold,
      config.riskAssessment.promptVersion,
      config.riskAssessment.maxAnalysisAttempts,
    );

    const notificationService = new NotificationService(
      createLidoGovernanceMonitorLogger("NotificationService"),
      slackClient,
      proposalRepository,
      config.riskAssessment.threshold,
      config.riskAssessment.maxNotifyAttempts,
    );

    return new LidoGovernanceMonitorBootstrap(
      bootstrapLogger,
      prisma,
      proposalFetcher,
      proposalProcessor,
      notificationService,
    );
  }

  async start(): Promise<void> {
    this.logger.info("Starting Lido Governance Monitor");

    try {
      await this.prisma.$connect();
      this.logger.info("Database connected");

      await this.proposalFetcher.getLatestProposals();
      await this.proposalProcessor.processOnce();
      await this.notificationService.notifyOnce();

      this.logger.info("Lido Governance Monitor execution completed");
    } catch (error) {
      this.logger.critical("Lido Governance Monitor execution failed", {
        error: error instanceof Error ? error.message : String(error),
      });
      throw error;
    }
  }

  async stop(): Promise<void> {
    this.logger.info("Stopping Lido Governance Monitor");
    await this.prisma.$disconnect();
    this.logger.info("Database disconnected");
  }
}
