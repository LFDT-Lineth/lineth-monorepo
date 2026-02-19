import "dotenv/config";

import { isAddress, getAddress } from "viem";
import { z } from "zod";

// Validation helpers that reject empty/whitespace-only strings
const NonEmptyString = (message: string) => z.string().trim().min(1, message);
const NonEmptyUrl = (message: string) => z.string().trim().url(message);

// Validates and normalizes an Ethereum address to checksummed form
const Address = z
  .string()
  .refine((v) => isAddress(v), { message: "Invalid Ethereum address" })
  .transform((v) => getAddress(v));

export const ConfigSchema = z.object({
  database: z.object({
    // PostgreSQL connection string for storing proposal lifecycle data
    url: NonEmptyString("Database URL is required"),
  }),
  discourse: z.object({
    // Lido Discourse forum API endpoint for fetching governance proposals
    proposalsUrl: NonEmptyUrl("Invalid Discourse proposals URL"),
    // Maximum number of Discourse topics to process per polling cycle
    maxTopicsPerPoll: z.number().int().positive("Max topics per poll must be positive"),
  }),
  anthropic: z.object({
    // Authentication key for the Anthropic Claude API
    apiKey: NonEmptyString("Anthropic API key is required"),
    // Claude model ID used for proposal risk analysis
    model: NonEmptyString("Model name is required"),
    // Maximum tokens Claude can generate per analysis response
    maxOutputTokens: z.number().int().positive("Max output tokens must be positive"),
    // Maximum proposal text length (in characters) sent to Claude for analysis
    maxProposalChars: z.number().int().positive("Max proposal chars must be positive"),
  }),
  slack: z.object({
    // Slack incoming webhook URL for sending high-risk proposal alerts
    webhookUrl: NonEmptyUrl("Invalid Slack webhook URL"),
    // Optional Slack webhook URL for logging all assessments regardless of risk score
    auditWebhookUrl: NonEmptyUrl("Invalid Slack audit webhook URL").optional(),
  }),
  riskAssessment: z.object({
    // Risk score (0-100) above which proposals trigger Slack alerts
    threshold: z.number().int().min(0).max(100, "Threshold must be 0-100"),
    // Version identifier for the risk assessment prompt, stored with each assessment
    promptVersion: NonEmptyString("Prompt version is required"),
    // Maximum number of AI analysis attempts before giving up on a proposal
    maxAnalysisAttempts: z.number().int().positive("Max analysis attempts must be positive").default(5),
    // Maximum number of Slack notification attempts before giving up on a proposal
    maxNotifyAttempts: z.number().int().positive("Max notify attempts must be positive").default(5),
  }),
  ethereum: z.object({
    // Ethereum mainnet RPC endpoint for reading on-chain voting data
    rpcUrl: NonEmptyUrl("Ethereum RPC URL is required"),
    // Address of the LDO voting contract to monitor
    ldoVotingContractAddress: Address,
    // Optional starting vote ID to skip historical votes on first run
    initialLdoVotingContractVoteId: z
      .number()
      .int()
      .nonnegative("Initial LDO voting contract vote ID must be non-negative")
      .optional(),
  }),
  http: z.object({
    // Timeout in milliseconds for external HTTP requests (Discourse and Slack)
    timeoutMs: z.number().int().positive("HTTP timeout must be positive"),
  }),
});

export type Config = z.infer<typeof ConfigSchema>;

export function loadConfigFromEnv(env: Record<string, string | undefined>): Config {
  const rawConfig = {
    database: {
      url: env.DATABASE_URL ?? "",
    },
    discourse: {
      proposalsUrl: env.DISCOURSE_PROPOSALS_URL ?? "",
      maxTopicsPerPoll: parseInt(env.MAX_TOPICS_PER_POLL ?? "20", 10),
    },
    anthropic: {
      apiKey: env.ANTHROPIC_API_KEY ?? "",
      model: env.CLAUDE_MODEL ?? "claude-sonnet-4-20250514",
      maxOutputTokens: parseInt(env.ANTHROPIC_MAX_OUTPUT_TOKENS ?? "4096", 10),
      maxProposalChars: parseInt(env.ANTHROPIC_MAX_PROPOSAL_CHARS ?? "700000", 10),
    },
    slack: {
      webhookUrl: env.SLACK_WEBHOOK_URL ?? "",
      auditWebhookUrl: env.SLACK_AUDIT_WEBHOOK_URL,
    },
    riskAssessment: {
      threshold: parseInt(env.RISK_THRESHOLD ?? "60", 10),
      promptVersion: env.PROMPT_VERSION ?? "v1.0",
      ...(env.MAX_ANALYSIS_ATTEMPTS ? { maxAnalysisAttempts: parseInt(env.MAX_ANALYSIS_ATTEMPTS, 10) } : {}),
      ...(env.MAX_NOTIFY_ATTEMPTS ? { maxNotifyAttempts: parseInt(env.MAX_NOTIFY_ATTEMPTS, 10) } : {}),
    },
    ethereum: {
      rpcUrl: env.ETHEREUM_RPC_URL ?? "",
      ldoVotingContractAddress: env.LDO_VOTING_CONTRACT_ADDRESS ?? "",
      initialLdoVotingContractVoteId: env.INITIAL_LDO_VOTING_CONTRACT_VOTEID
        ? parseInt(env.INITIAL_LDO_VOTING_CONTRACT_VOTEID, 10)
        : undefined,
    },
    http: {
      timeoutMs: parseInt(env.HTTP_TIMEOUT_MS ?? "15000", 10),
    },
  };

  const result = ConfigSchema.safeParse(rawConfig);
  if (!result.success) {
    const errors = result.error.errors.map((e) => `${e.path.join(".")}: ${e.message}`).join(", ");
    throw new Error(`Configuration validation failed: ${errors}`);
  }

  return result.data;
}
