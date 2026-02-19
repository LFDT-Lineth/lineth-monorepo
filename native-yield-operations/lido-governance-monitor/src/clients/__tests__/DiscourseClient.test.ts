import { IRetryService } from "@consensys/linea-shared-utils";
import { jest, describe, it, expect, beforeEach } from "@jest/globals";

import { ILidoGovernanceMonitorLogger } from "../../utils/logging/index.js";
import { DiscourseClient } from "../DiscourseClient.js";

const createLoggerMock = (): jest.Mocked<ILidoGovernanceMonitorLogger> => ({
  name: "test-logger",
  debug: jest.fn(),
  error: jest.fn(),
  info: jest.fn(),
  warn: jest.fn(),
  critical: jest.fn(),
});

const createRetryServiceMock = (): jest.Mocked<IRetryService> => ({
  retry: jest.fn().mockImplementation(<T>(fn: () => Promise<T>) => fn()),
});

describe("DiscourseClient", () => {
  let client: DiscourseClient;
  let logger: jest.Mocked<ILidoGovernanceMonitorLogger>;
  let retryService: jest.Mocked<IRetryService>;
  let fetchMock: jest.Mock;

  beforeEach(() => {
    logger = createLoggerMock();
    retryService = createRetryServiceMock();
    fetchMock = jest.fn();
    global.fetch = fetchMock as unknown as typeof fetch;
    client = new DiscourseClient(logger, retryService, "https://research.lido.fi/c/proposals/9/l/latest.json", 15000);
  });

  describe("getBaseUrl", () => {
    it("returns the base URL (origin extracted from proposals URL)", () => {
      // Act
      const baseUrl = client.getBaseUrl();

      // Assert
      expect(baseUrl).toBe("https://research.lido.fi");
    });
  });

  describe("fetchLatestProposals", () => {
    it("fetches and returns latest proposals from Discourse API", async () => {
      // Arrange
      const mockResponse = { topic_list: { topics: [{ id: 11107, slug: "test-proposal" }] } };
      fetchMock.mockResolvedValue({ ok: true, json: () => Promise.resolve(mockResponse) });

      // Act
      const result = await client.fetchLatestProposals();

      // Assert
      expect(result).toEqual(mockResponse);
      expect(fetchMock).toHaveBeenCalledWith("https://research.lido.fi/c/proposals/9/l/latest.json");
    });

    it("returns undefined on fetch failure and logs critical", async () => {
      // Arrange
      fetchMock.mockResolvedValue({ ok: false, status: 500, statusText: "Internal Server Error" });

      // Act
      const result = await client.fetchLatestProposals();

      // Assert
      expect(result).toBeUndefined();
      expect(logger.critical).toHaveBeenCalledWith("Failed to fetch latest proposals", {
        status: 500,
        statusText: "Internal Server Error",
      });
    });

    it("returns undefined on network error and logs critical", async () => {
      // Arrange
      const networkError = new Error("Network error");
      fetchMock.mockRejectedValue(networkError);

      // Act
      const result = await client.fetchLatestProposals();

      // Assert
      expect(result).toBeUndefined();
      expect(logger.critical).toHaveBeenCalledWith("Error fetching latest proposals", { error: networkError });
    });

    it("returns undefined when API response fails schema validation", async () => {
      // Arrange - return data that violates RawDiscourseProposalListSchema
      const invalidResponse = {
        wrong_field: "this doesn't match the schema",
        // Missing required 'topic_list' field
      };
      fetchMock.mockResolvedValue({ ok: true, json: () => Promise.resolve(invalidResponse) });

      // Act
      const result = await client.fetchLatestProposals();

      // Assert
      expect(result).toBeUndefined();
      expect(logger.error).toHaveBeenCalledWith(
        "Discourse API response failed schema validation",
        expect.objectContaining({ errors: expect.any(Array) }),
      );
    });

    it("uses retry service for fetch call", async () => {
      // Arrange
      const mockResponse = { topic_list: { topics: [] } };
      fetchMock.mockResolvedValue({ ok: true, json: () => Promise.resolve(mockResponse) });

      // Act
      await client.fetchLatestProposals();

      // Assert
      expect(retryService.retry).toHaveBeenCalledTimes(1);
      expect(retryService.retry).toHaveBeenCalledWith(expect.any(Function), 15000);
    });
  });

  describe("fetchProposalDetails", () => {
    it("fetches and returns proposal details", async () => {
      // Arrange
      const mockProposal = {
        id: 11107,
        slug: "test-proposal",
        title: "Test Proposal",
        created_at: "2024-01-01T00:00:00.000Z",
        post_stream: {
          posts: [
            {
              id: 24002,
              username: "testuser",
              cooked: "<p>Content</p>",
              post_url: "/t/test/1",
              created_at: "2024-01-01",
            },
          ],
        },
      };
      fetchMock.mockResolvedValue({ ok: true, json: () => Promise.resolve(mockProposal) });

      // Act
      const result = await client.fetchProposalDetails(11107);

      // Assert
      expect(result).toEqual(mockProposal);
      expect(fetchMock).toHaveBeenCalledWith("https://research.lido.fi/t/11107.json");
    });

    it("returns undefined on fetch failure and logs critical", async () => {
      // Arrange
      fetchMock.mockResolvedValue({ ok: false, status: 404, statusText: "Not Found" });

      // Act
      const result = await client.fetchProposalDetails(99999);

      // Assert
      expect(result).toBeUndefined();
      expect(logger.critical).toHaveBeenCalledWith("Failed to fetch proposal details", {
        proposalId: 99999,
        status: 404,
      });
    });

    it("returns undefined on network error and logs critical", async () => {
      // Arrange
      const networkError = new Error("Network error");
      fetchMock.mockRejectedValue(networkError);

      // Act
      const result = await client.fetchProposalDetails(11107);

      // Assert
      expect(result).toBeUndefined();
      expect(logger.critical).toHaveBeenCalledWith("Error fetching proposal details", {
        proposalId: 11107,
        error: networkError,
      });
    });

    it("uses retry service for fetch call", async () => {
      // Arrange
      const mockProposal = { id: 11107, title: "Test" };
      fetchMock.mockResolvedValue({ ok: true, json: () => Promise.resolve(mockProposal) });

      // Act
      await client.fetchProposalDetails(11107);

      // Assert
      expect(retryService.retry).toHaveBeenCalledTimes(1);
      expect(retryService.retry).toHaveBeenCalledWith(expect.any(Function), 15000);
    });
  });
});
