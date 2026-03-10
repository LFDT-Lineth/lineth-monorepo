export function computeEffectiveRisk(riskScore: number, confidence: number, effectiveRisk?: number): number {
  return effectiveRisk ?? Math.round((riskScore * confidence) / 100);
}
