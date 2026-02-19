import { jest, describe, it, expect, beforeEach } from "@jest/globals";

import { IDiscourseClient } from "../../../core/clients/IDiscourseClient.js";
import { ProposalSource } from "../../../core/entities/ProposalSource.js";
import { RawDiscourseProposal, RawDiscourseProposalList } from "../../../core/entities/RawDiscourseProposal.js";
import { IProposalRepository } from "../../../core/repositories/IProposalRepository.js";
import { INormalizationService } from "../../../core/services/INormalizationService.js";
import { ILidoGovernanceMonitorLogger } from "../../../utils/logging/index.js";
import { DiscourseFetcher } from "../../fetchers/DiscourseFetcher.js";

const createLoggerMock = (): jest.Mocked<ILidoGovernanceMonitorLogger> => ({
  name: "test-logger",
  critical: jest.fn(),
  debug: jest.fn(),
  error: jest.fn(),
  info: jest.fn(),
  warn: jest.fn(),
});

const createProposalRepositoryMock = (): jest.Mocked<Pick<IProposalRepository, "upsert">> => ({
  upsert: jest.fn(),
});

describe("DiscourseFetcher", () => {
  let fetcher: DiscourseFetcher;
  let logger: jest.Mocked<ILidoGovernanceMonitorLogger>;
  let discourseClient: jest.Mocked<IDiscourseClient>;
  let normalizationService: jest.Mocked<INormalizationService>;
  let proposalRepository: jest.Mocked<Pick<IProposalRepository, "upsert">>;

  const createMockProposalList = (topics: Array<{ id: number; slug: string }>): RawDiscourseProposalList => ({
    topic_list: { topics },
  });

  const createMockProposal = (id: number, slug: string): RawDiscourseProposal => ({
    id,
    slug,
    title: `Proposal ${id}`,
    created_at: "2024-01-15T10:00:00.000Z",
    post_stream: {
      posts: [{ id: 1, username: "author", cooked: "<p>Content</p>", post_url: "/t/1", created_at: "2024-01-15" }],
    },
  });

  const createNormalizedInput = (sourceId: string) => ({
    source: ProposalSource.DISCOURSE,
    sourceId,
    url: `https://research.lido.fi/t/proposal/${sourceId}`,
    title: `Proposal ${sourceId}`,
    author: "author",
    sourceCreatedAt: new Date("2024-01-15"),
    rawProposalText: "Content",
  });

  beforeEach(() => {
    logger = createLoggerMock();
    discourseClient = {
      fetchLatestProposals: jest.fn(),
      fetchProposalDetails: jest.fn(),
      getBaseUrl: jest.fn(),
    } as jest.Mocked<IDiscourseClient>;
    normalizationService = {
      normalizeDiscourseProposal: jest.fn(),
      stripHtml: jest.fn(),
    } as jest.Mocked<INormalizationService>;
    proposalRepository = createProposalRepositoryMock();
    proposalRepository.upsert.mockResolvedValue({ proposal: { id: "uuid" } as never, isNew: true });

    fetcher = new DiscourseFetcher(
      logger,
      discourseClient,
      normalizationService,
      proposalRepository as unknown as IProposalRepository,
      20,
    );
  });

  describe("getLatestProposals", () => {
    it("fetches proposals, normalizes, and returns CreateProposalInput[]", async () => {
      // Arrange
      const topicList = createMockProposalList([{ id: 100, slug: "proposal-a" }]);
      const proposalDetails = createMockProposal(100, "proposal-a");
      const normalizedInput = createNormalizedInput("100");

      discourseClient.fetchLatestProposals.mockResolvedValue(topicList);
      discourseClient.fetchProposalDetails.mockResolvedValue(proposalDetails);
      normalizationService.normalizeDiscourseProposal.mockReturnValue(normalizedInput);

      // Act
      const result = await fetcher.getLatestProposals();

      // Assert
      expect(result).toEqual([normalizedInput]);
      expect(discourseClient.fetchLatestProposals).toHaveBeenCalled();
      expect(discourseClient.fetchProposalDetails).toHaveBeenCalledWith(100);
      expect(normalizationService.normalizeDiscourseProposal).toHaveBeenCalledWith(proposalDetails);
    });

    it("returns empty array when fetchLatestProposals returns undefined", async () => {
      // Arrange
      discourseClient.fetchLatestProposals.mockResolvedValue(undefined);

      // Act
      const result = await fetcher.getLatestProposals();

      // Assert
      expect(result).toEqual([]);
      expect(logger.warn).toHaveBeenCalledWith("Failed to fetch latest proposals from Discourse");
    });

    it("limits topics to maxTopicsPerPoll", async () => {
      // Arrange
      const topics = Array.from({ length: 5 }, (_, i) => ({ id: i + 1, slug: `proposal-${i + 1}` }));
      const topicList = createMockProposalList(topics);

      discourseClient.fetchLatestProposals.mockResolvedValue(topicList);
      discourseClient.fetchProposalDetails.mockResolvedValue(undefined);

      // Create fetcher with maxTopicsPerPoll=2
      const limitedFetcher = new DiscourseFetcher(
        logger,
        discourseClient,
        normalizationService,
        proposalRepository as unknown as IProposalRepository,
        2,
      );

      // Act
      await limitedFetcher.getLatestProposals();

      // Assert
      expect(discourseClient.fetchProposalDetails).toHaveBeenCalledTimes(2);
      expect(discourseClient.fetchProposalDetails).toHaveBeenCalledWith(1);
      expect(discourseClient.fetchProposalDetails).toHaveBeenCalledWith(2);
    });

    it("skips topics where details fetch fails and continues with others", async () => {
      // Arrange
      const topicList = createMockProposalList([
        { id: 100, slug: "fails" },
        { id: 101, slug: "succeeds" },
      ]);
      const proposalDetails = createMockProposal(101, "succeeds");
      const normalizedInput = createNormalizedInput("101");

      discourseClient.fetchLatestProposals.mockResolvedValue(topicList);
      discourseClient.fetchProposalDetails.mockResolvedValueOnce(undefined).mockResolvedValueOnce(proposalDetails);
      normalizationService.normalizeDiscourseProposal.mockReturnValue(normalizedInput);

      // Act
      const result = await fetcher.getLatestProposals();

      // Assert
      expect(result).toEqual([normalizedInput]);
      expect(logger.warn).toHaveBeenCalledWith("Failed to fetch proposal details", { topicId: 100 });
    });

    it("skips topics where normalization throws and continues with others", async () => {
      // Arrange
      const topicList = createMockProposalList([
        { id: 100, slug: "bad-normalize" },
        { id: 101, slug: "good-normalize" },
      ]);
      const badDetails = createMockProposal(100, "bad-normalize");
      const goodDetails = createMockProposal(101, "good-normalize");
      const normalizedInput = createNormalizedInput("101");

      discourseClient.fetchLatestProposals.mockResolvedValue(topicList);
      discourseClient.fetchProposalDetails.mockResolvedValueOnce(badDetails).mockResolvedValueOnce(goodDetails);
      normalizationService.normalizeDiscourseProposal
        .mockImplementationOnce(() => {
          throw new Error("Normalization error");
        })
        .mockReturnValueOnce(normalizedInput);

      // Act
      const result = await fetcher.getLatestProposals();

      // Assert
      expect(result).toEqual([normalizedInput]);
      expect(logger.error).toHaveBeenCalledWith(
        "Failed to normalize proposal",
        expect.objectContaining({ topicId: 100 }),
      );
    });

    it("calls upsert for each successfully normalized proposal", async () => {
      // Arrange
      const topicList = createMockProposalList([
        { id: 100, slug: "proposal-a" },
        { id: 101, slug: "proposal-b" },
      ]);
      const detailsA = createMockProposal(100, "proposal-a");
      const detailsB = createMockProposal(101, "proposal-b");
      const normalizedA = createNormalizedInput("100");
      const normalizedB = createNormalizedInput("101");

      discourseClient.fetchLatestProposals.mockResolvedValue(topicList);
      discourseClient.fetchProposalDetails.mockResolvedValueOnce(detailsA).mockResolvedValueOnce(detailsB);
      normalizationService.normalizeDiscourseProposal.mockReturnValueOnce(normalizedA).mockReturnValueOnce(normalizedB);

      // Act
      await fetcher.getLatestProposals();

      // Assert
      expect(proposalRepository.upsert).toHaveBeenCalledTimes(2);
      expect(proposalRepository.upsert).toHaveBeenCalledWith(normalizedA);
      expect(proposalRepository.upsert).toHaveBeenCalledWith(normalizedB);
    });

    it("logs info when upsert reports isNew true", async () => {
      // Arrange
      const topicList = createMockProposalList([{ id: 100, slug: "proposal-a" }]);
      const details = createMockProposal(100, "proposal-a");
      const normalized = createNormalizedInput("100");

      discourseClient.fetchLatestProposals.mockResolvedValue(topicList);
      discourseClient.fetchProposalDetails.mockResolvedValue(details);
      normalizationService.normalizeDiscourseProposal.mockReturnValue(normalized);
      proposalRepository.upsert.mockResolvedValue({ proposal: { id: "new-uuid" } as never, isNew: true });

      // Act
      await fetcher.getLatestProposals();

      // Assert
      expect(logger.info).toHaveBeenCalledWith("Created new Discourse proposal", { sourceId: "100" });
    });

    it("logs debug when upsert reports isNew false", async () => {
      // Arrange
      const topicList = createMockProposalList([{ id: 100, slug: "proposal-a" }]);
      const details = createMockProposal(100, "proposal-a");
      const normalized = createNormalizedInput("100");

      discourseClient.fetchLatestProposals.mockResolvedValue(topicList);
      discourseClient.fetchProposalDetails.mockResolvedValue(details);
      normalizationService.normalizeDiscourseProposal.mockReturnValue(normalized);
      proposalRepository.upsert.mockResolvedValue({ proposal: { id: "existing-uuid" } as never, isNew: false });

      // Act
      await fetcher.getLatestProposals();

      // Assert
      expect(logger.debug).toHaveBeenCalledWith("Discourse proposal already exists, skipping", { sourceId: "100" });
    });

    it("continues processing remaining topics when upsert throws", async () => {
      // Arrange
      const topicList = createMockProposalList([
        { id: 100, slug: "upsert-fails" },
        { id: 101, slug: "upsert-succeeds" },
      ]);
      const detailsA = createMockProposal(100, "upsert-fails");
      const detailsB = createMockProposal(101, "upsert-succeeds");
      const normalizedA = createNormalizedInput("100");
      const normalizedB = createNormalizedInput("101");

      discourseClient.fetchLatestProposals.mockResolvedValue(topicList);
      discourseClient.fetchProposalDetails.mockResolvedValueOnce(detailsA).mockResolvedValueOnce(detailsB);
      normalizationService.normalizeDiscourseProposal.mockReturnValueOnce(normalizedA).mockReturnValueOnce(normalizedB);
      proposalRepository.upsert
        .mockRejectedValueOnce(new Error("DB error"))
        .mockResolvedValueOnce({ proposal: { id: "uuid" } as never, isNew: true });

      // Act
      const result = await fetcher.getLatestProposals();

      // Assert - both proposals returned despite first upsert failing
      expect(result).toEqual([normalizedA, normalizedB]);
      expect(logger.critical).toHaveBeenCalledWith("Failed to persist Discourse proposal", {
        sourceId: "100",
        error: "DB error",
      });
      expect(proposalRepository.upsert).toHaveBeenCalledTimes(2);
    });

    it("still returns the normalized proposal even when upsert fails", async () => {
      // Arrange
      const topicList = createMockProposalList([{ id: 100, slug: "proposal-a" }]);
      const details = createMockProposal(100, "proposal-a");
      const normalized = createNormalizedInput("100");

      discourseClient.fetchLatestProposals.mockResolvedValue(topicList);
      discourseClient.fetchProposalDetails.mockResolvedValue(details);
      normalizationService.normalizeDiscourseProposal.mockReturnValue(normalized);
      proposalRepository.upsert.mockRejectedValue(new Error("DB error"));

      // Act
      const result = await fetcher.getLatestProposals();

      // Assert - proposal is still in the returned array
      expect(result).toEqual([normalized]);
    });
  });
});
