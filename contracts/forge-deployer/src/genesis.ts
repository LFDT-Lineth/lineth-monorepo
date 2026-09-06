export interface GenesisBlockProvider {
  getBlock(blockNumber: number): Promise<{ timestamp: number } | null>;
}

function parseTimestamp(raw: string): number {
  if (!/^[0-9]+$/.test(raw)) throw new Error("L2_GENESIS_TIMESTAMP must be a non-negative integer");
  const timestamp = Number(raw);
  if (!Number.isSafeInteger(timestamp)) throw new Error("L2_GENESIS_TIMESTAMP must be a safe non-negative integer");
  return timestamp;
}

export async function resolveGenesisTimestamp(
  provider: GenesisBlockProvider,
  override: string | undefined,
): Promise<number> {
  if (override !== undefined) return parseTimestamp(override);

  const genesisBlock = await provider.getBlock(0);
  if (!genesisBlock) throw new Error("L2 RPC did not return genesis block 0");
  return parseTimestamp(String(genesisBlock.timestamp));
}
