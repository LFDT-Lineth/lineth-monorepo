import { z } from "zod";

export type ImpactType = "economic" | "technical" | "operational" | "governance-process";
export type RiskLevel = "low" | "medium" | "high" | "critical";
export type RecommendedAction = "no-action" | "monitor" | "comment" | "escalate";
export type ProposalType = "discourse" | "snapshot" | "onchain_vote";
export type Urgency = "none" | "routine" | "urgent" | "critical";
export type AffectedComponent =
  | "StakingVault"
  | "VaultHub"
  | "LazyOracle"
  | "OperatorGrid"
  | "PredepositGuarantee"
  | "Dashboard"
  | "Other";
export const NativeYieldInvariant = {
  VALID_YIELD_REPORTING: "Valid yield reporting",
  USER_PRINCIPAL_PROTECTION: "User principal protection",
  PAUSE_DEPOSITS: "Pause deposits only on deficit, or liability or ossification",
  OTHER: "Other",
} as const;
export type NativeYieldInvariant = (typeof NativeYieldInvariant)[keyof typeof NativeYieldInvariant];

export interface Assessment {
  riskScore: number;
  riskLevel: RiskLevel;
  confidence: number;
  proposalType: ProposalType;
  impactTypes: ImpactType[];
  affectedComponents: AffectedComponent[];
  whatChanged: string;
  nativeYieldInvariantsAtRisk: NativeYieldInvariant[];
  nativeYieldImpact: string[];
  recommendedAction: RecommendedAction;
  urgency: Urgency;
  supportingQuotes: string[];
  keyUnknowns: string[];
}

// Zod schema for validating Assessment objects deserialized from JSON (e.g. DB round-trips).
// Mirrors the Assessment interface above so that stale or malformed data is caught at runtime.
export const AssessmentSchema = z.object({
  riskScore: z.number(),
  riskLevel: z.enum(["low", "medium", "high", "critical"]),
  confidence: z.number(),
  proposalType: z.enum(["discourse", "snapshot", "onchain_vote"]),
  impactTypes: z.array(z.enum(["economic", "technical", "operational", "governance-process"])),
  affectedComponents: z.array(
    z.enum(["StakingVault", "VaultHub", "LazyOracle", "OperatorGrid", "PredepositGuarantee", "Dashboard", "Other"]),
  ),
  whatChanged: z.string(),
  nativeYieldInvariantsAtRisk: z.array(
    z.enum([
      NativeYieldInvariant.VALID_YIELD_REPORTING,
      NativeYieldInvariant.USER_PRINCIPAL_PROTECTION,
      NativeYieldInvariant.PAUSE_DEPOSITS,
      NativeYieldInvariant.OTHER,
    ]),
  ),
  nativeYieldImpact: z.array(z.string()),
  recommendedAction: z.enum(["no-action", "monitor", "comment", "escalate"]),
  urgency: z.enum(["none", "routine", "urgent", "critical"]),
  supportingQuotes: z.array(z.string()),
  keyUnknowns: z.array(z.string()),
});
