import { jest, describe, it, expect, beforeEach } from "@jest/globals";

import { ProposalSource } from "../../core/entities/ProposalSource.js";
import { RawDiscourseProposal } from "../../core/entities/RawDiscourseProposal.js";
import { ILidoGovernanceMonitorLogger } from "../../utils/logging/index.js";
import { NormalizationService } from "../NormalizationService.js";

const createLoggerMock = (): jest.Mocked<ILidoGovernanceMonitorLogger> => ({
  name: "test-logger",
  debug: jest.fn(),
  error: jest.fn(),
  info: jest.fn(),
  warn: jest.fn(),
  critical: jest.fn(),
});

describe("NormalizationService", () => {
  let service: NormalizationService;
  let logger: jest.Mocked<ILidoGovernanceMonitorLogger>;

  beforeEach(() => {
    logger = createLoggerMock();
    service = new NormalizationService(logger, "https://research.lido.fi");
  });

  describe("stripHtml", () => {
    it("removes HTML tags and preserves text content", () => {
      // Arrange
      const html = "<p>Hello <strong>world</strong></p>";

      // Act
      const result = service.stripHtml(html);

      // Assert
      expect(result).toBe("Hello world");
    });

    it("decodes HTML entities", () => {
      // Arrange
      const html = "<p>Hello &amp; goodbye &lt;world&gt;</p>";

      // Act
      const result = service.stripHtml(html);

      // Assert
      expect(result).toBe("Hello & goodbye <world>");
    });

    it("handles nested tags", () => {
      // Arrange
      const html = "<div><ul><li>Item 1</li><li>Item 2</li></ul></div>";

      // Act
      const result = service.stripHtml(html);

      // Assert
      expect(result).toContain("Item 1");
      expect(result).toContain("Item 2");
    });

    it("converts block elements to newlines", () => {
      // Arrange
      const html = "<p>Paragraph 1</p><p>Paragraph 2</p>";

      // Act
      const result = service.stripHtml(html);

      // Assert
      expect(result).toContain("Paragraph 1");
      expect(result).toContain("Paragraph 2");
    });

    it("handles empty string", () => {
      // Arrange & Act
      const result = service.stripHtml("");

      // Assert
      expect(result).toBe("");
    });

    it("decodes quote and apostrophe entities", () => {
      // Arrange
      const html = "<p>It&#39;s a &quot;test&quot;</p>";

      // Act
      const result = service.stripHtml(html);

      // Assert
      expect(result).toBe('It\'s a "test"');
    });
  });

  describe("normalizeDiscourseProposal", () => {
    const createMockProposal = (overrides: Partial<RawDiscourseProposal> = {}): RawDiscourseProposal => ({
      id: 11107,
      slug: "test-proposal-title",
      title: "Test Proposal Title",
      created_at: "2024-01-15T10:30:00.000Z",
      post_stream: {
        posts: [
          {
            id: 24002,
            username: "testuser",
            cooked: "<p>This is the proposal content.</p>",
            post_url: "/t/test-proposal-title/11107/1",
            created_at: "2024-01-15T10:30:00.000Z",
          },
        ],
      },
      ...overrides,
    });

    it("converts Discourse proposal to CreateProposalInput with correct source", () => {
      // Arrange
      const proposal = createMockProposal();

      // Act
      const result = service.normalizeDiscourseProposal(proposal);

      // Assert
      expect(result.source).toBe(ProposalSource.DISCOURSE);
    });

    it("extracts sourceId from topic id", () => {
      // Arrange
      const proposal = createMockProposal({ id: 12345 });

      // Act
      const result = service.normalizeDiscourseProposal(proposal);

      // Assert
      expect(result.sourceId).toBe("12345");
    });

    it("builds correct URL from baseUrl, slug and id", () => {
      // Arrange
      const proposal = createMockProposal({ id: 11107, slug: "my-proposal" });

      // Act
      const result = service.normalizeDiscourseProposal(proposal);

      // Assert
      expect(result.url).toBe("https://research.lido.fi/t/my-proposal/11107");
    });

    it("extracts title from proposal", () => {
      // Arrange
      const proposal = createMockProposal({ title: "Important Governance Proposal" });

      // Act
      const result = service.normalizeDiscourseProposal(proposal);

      // Assert
      expect(result.title).toBe("Important Governance Proposal");
    });

    it("extracts author from first post username", () => {
      // Arrange
      const proposal = createMockProposal();
      proposal.post_stream.posts[0].username = "governance_lead";

      // Act
      const result = service.normalizeDiscourseProposal(proposal);

      // Assert
      expect(result.author).toBe("governance_lead");
    });

    it("sets author to null when no posts exist", () => {
      // Arrange
      const proposal = createMockProposal();
      proposal.post_stream.posts = [];

      // Act
      const result = service.normalizeDiscourseProposal(proposal);

      // Assert
      expect(result.author).toBeNull();
    });

    it("parses sourceCreatedAt from created_at field", () => {
      // Arrange
      const proposal = createMockProposal({ created_at: "2024-06-15T14:30:00.000Z" });

      // Act
      const result = service.normalizeDiscourseProposal(proposal);

      // Assert
      expect(result.sourceCreatedAt).toEqual(new Date("2024-06-15T14:30:00.000Z"));
    });

    it("strips HTML from post content and joins multiple posts", () => {
      // Arrange
      const proposal = createMockProposal();
      proposal.post_stream.posts = [
        { id: 1, username: "user1", cooked: "<p>First post content.</p>", post_url: "/t/1", created_at: "2024-01-01" },
        { id: 2, username: "user2", cooked: "<p>Second post content.</p>", post_url: "/t/2", created_at: "2024-01-02" },
      ];

      // Act
      const result = service.normalizeDiscourseProposal(proposal);

      // Assert
      expect(result.rawProposalText).toContain("First post content.");
      expect(result.rawProposalText).toContain("Second post content.");
    });

    it("returns empty text when no posts exist", () => {
      // Arrange
      const proposal = createMockProposal();
      proposal.post_stream.posts = [];

      // Act
      const result = service.normalizeDiscourseProposal(proposal);

      // Assert
      expect(result.rawProposalText).toBe("");
    });

    it("logs debug message after normalization", () => {
      // Arrange
      const proposal = createMockProposal();

      // Act
      service.normalizeDiscourseProposal(proposal);

      // Assert
      expect(logger.debug).toHaveBeenCalledWith("Normalized Discourse proposal", expect.any(Object));
    });
  });
});
