import { htmlToText } from "html-to-text";

import { CreateProposalInput } from "../core/entities/Proposal.js";
import { ProposalSource } from "../core/entities/ProposalSource.js";
import { RawDiscourseProposal } from "../core/entities/RawDiscourseProposal.js";
import { INormalizationService } from "../core/services/INormalizationService.js";
import { ILidoGovernanceMonitorLogger } from "../utils/logging/index.js";

export class NormalizationService implements INormalizationService {
  constructor(
    private readonly logger: ILidoGovernanceMonitorLogger,
    private readonly discourseBaseUrl: string,
  ) {}

  normalizeDiscourseProposal(proposal: RawDiscourseProposal): CreateProposalInput {
    const posts = proposal.post_stream.posts;
    const firstPost = posts[0];
    const text = posts.map((post) => this.stripHtml(post.cooked)).join("\n\n");

    const input: CreateProposalInput = {
      source: ProposalSource.DISCOURSE,
      sourceId: proposal.id.toString(),
      url: `${this.discourseBaseUrl}/t/${proposal.slug}/${proposal.id}`,
      title: proposal.title,
      author: firstPost?.username ?? null,
      sourceCreatedAt: new Date(proposal.created_at),
      rawProposalText: text.trim(),
    };

    this.logger.debug("Normalized Discourse proposal", {
      sourceId: input.sourceId,
      title: input.title,
      textLength: input.rawProposalText.length,
    });

    return input;
  }

  stripHtml(html: string): string {
    if (!html) return "";

    // Convert HTML to plain text using html-to-text library
    const text = htmlToText(html, {
      wordwrap: false, // Preserve original line structure
      selectors: [
        // Block elements create newlines
        { selector: "p", options: { leadingLineBreaks: 1, trailingLineBreaks: 1 } },
        { selector: "div", options: { leadingLineBreaks: 1, trailingLineBreaks: 1 } },
        { selector: "br", options: { leadingLineBreaks: 1, trailingLineBreaks: 0 } },
        { selector: "li", format: "block", options: { leadingLineBreaks: 1, trailingLineBreaks: 0 } },
        { selector: "h1", options: { leadingLineBreaks: 1, trailingLineBreaks: 1 } },
        { selector: "h2", options: { leadingLineBreaks: 1, trailingLineBreaks: 1 } },
        { selector: "h3", options: { leadingLineBreaks: 1, trailingLineBreaks: 1 } },
        { selector: "h4", options: { leadingLineBreaks: 1, trailingLineBreaks: 1 } },
        { selector: "h5", options: { leadingLineBreaks: 1, trailingLineBreaks: 1 } },
        { selector: "h6", options: { leadingLineBreaks: 1, trailingLineBreaks: 1 } },
        // Remove unwanted elements entirely
        { selector: "script", format: "skip" },
        { selector: "style", format: "skip" },
      ],
    });

    // Normalize whitespace (same as before)
    return text.replace(/\n{3,}/g, "\n\n").trim();
  }
}
