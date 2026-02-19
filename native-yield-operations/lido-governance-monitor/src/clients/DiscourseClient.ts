import { IRetryService } from "@consensys/linea-shared-utils";

import { IDiscourseClient } from "../core/clients/IDiscourseClient.js";
import {
  RawDiscourseProposal,
  RawDiscourseProposalList,
  RawDiscourseProposalListSchema,
  RawDiscourseProposalSchema,
} from "../core/entities/RawDiscourseProposal.js";
import { ILidoGovernanceMonitorLogger } from "../utils/logging/index.js";

export class DiscourseClient implements IDiscourseClient {
  private readonly baseUrl: string;

  constructor(
    private readonly logger: ILidoGovernanceMonitorLogger,
    private readonly retryService: IRetryService,
    private readonly proposalsUrl: string,
    private readonly httpTimeoutMs: number,
  ) {
    // Derive base URL from proposals URL for fetching individual proposals
    this.baseUrl = new URL(proposalsUrl).origin;
  }

  async fetchLatestProposals(): Promise<RawDiscourseProposalList | undefined> {
    const url = this.proposalsUrl;
    try {
      const response = await this.retryService.retry(() => fetch(url), this.httpTimeoutMs);
      if (!response.ok) {
        this.logger.critical("Failed to fetch latest proposals", {
          status: response.status,
          statusText: response.statusText,
        });
        return undefined;
      }
      const data = await response.json();

      // Validate API response with non-strict schema
      const validationResult = RawDiscourseProposalListSchema.safeParse(data);
      if (!validationResult.success) {
        this.logger.error("Discourse API response failed schema validation", {
          errors: validationResult.error.errors,
        });
        return undefined;
      }

      this.logger.debug("Fetched latest proposals", { count: validationResult.data.topic_list.topics.length });
      return validationResult.data;
    } catch (error) {
      this.logger.critical("Error fetching latest proposals", { error });
      return undefined;
    }
  }

  getBaseUrl(): string {
    return this.baseUrl;
  }

  async fetchProposalDetails(proposalId: number): Promise<RawDiscourseProposal | undefined> {
    const url = `${this.baseUrl}/t/${proposalId}.json`;
    try {
      const response = await this.retryService.retry(() => fetch(url), this.httpTimeoutMs);
      if (!response.ok) {
        this.logger.critical("Failed to fetch proposal details", {
          proposalId,
          status: response.status,
        });
        return undefined;
      }
      const data = await response.json();

      // Validate API response with non-strict schema
      const validationResult = RawDiscourseProposalSchema.safeParse(data);
      if (!validationResult.success) {
        this.logger.error("Discourse API response failed schema validation", {
          proposalId,
          errors: validationResult.error.errors,
        });
        return undefined;
      }

      this.logger.debug("Fetched proposal details", { proposalId, title: validationResult.data.title });
      return validationResult.data;
    } catch (error) {
      this.logger.critical("Error fetching proposal details", { proposalId, error });
      return undefined;
    }
  }
}
