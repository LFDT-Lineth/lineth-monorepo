import { z } from "zod";

// ============================================================================
// Request Validation Schemas
// ============================================================================

export const adapterSchema = z.enum(["ethers", "viem"]);

export const verificationOptionsSchema = z.object({
  verbose: z.boolean().default(false),
  skipBytecode: z.boolean().default(false),
  skipAbi: z.boolean().default(false),
  skipState: z.boolean().default(false),
  contractFilter: z.string().optional(),
  chainFilter: z.string().optional(),
});

export const verifyRequestSchema = z.object({
  sessionId: z.string().uuid(),
  adapter: adapterSchema,
  envVars: z.record(z.string(), z.string()),
  options: verificationOptionsSchema,
});

export const uploadTypeSchema = z.enum(["config", "schema", "artifact"]);

// ============================================================================
// Config Validation
// ============================================================================

export const chainConfigSchema = z.object({
  chainId: z.number(),
  rpcUrl: z.string(),
  explorerUrl: z.string().optional(),
});

export const stateVerificationSchema = z.object({
  ozVersion: z.enum(["v4", "v5", "auto"]).optional(),
  schemaFile: z.string().optional(),
  viewCalls: z.array(z.any()).optional(),
  namespaces: z.array(z.any()).optional(),
  slots: z.array(z.any()).optional(),
  storagePaths: z.array(z.any()).optional(),
});

export const contractConfigSchema = z.object({
  name: z.string(),
  chain: z.string(),
  address: z.string(),
  artifactFile: z.string(),
  isProxy: z.boolean().optional(),
  constructorArgs: z.union([z.array(z.any()), z.string()]).optional(),
  stateVerification: stateVerificationSchema.optional(),
});

export const verifierConfigSchema = z.object({
  chains: z.record(z.string(), chainConfigSchema),
  contracts: z.array(contractConfigSchema),
});

// ============================================================================
// Validation Utilities
// ============================================================================

export function isValidAddress(value: string): boolean {
  return /^0x[a-fA-F0-9]{40}$/.test(value);
}

export function isValidUrl(value: string): boolean {
  try {
    new URL(value);
    return true;
  } catch {
    return false;
  }
}

export function isValidHex(value: string): boolean {
  return /^0x[a-fA-F0-9]*$/.test(value);
}

// ============================================================================
// Path Validation
// ============================================================================

const DANGEROUS_PATTERNS = [
  /\.\./, // Path traversal
  /^\/etc\//, // System directories
  /^\/var\//,
  /^\/usr\//,
  /^\/bin\//,
  /^\/sbin\//,
  /^~\//, // Home expansion
  /\$\{/, // Variable expansion
  /\$\(/, // Command substitution
];

export function isPathSafe(filepath: string): boolean {
  const normalized = filepath.trim();

  for (const pattern of DANGEROUS_PATTERNS) {
    if (pattern.test(normalized)) {
      return false;
    }
  }

  // Only allow alphanumeric, dash, underscore, dot, and forward slash
  if (!/^[\w\-./]+$/.test(normalized)) {
    return false;
  }

  return true;
}

export function sanitizeFilename(filename: string): string {
  // Extract just the filename, removing any path components
  const basename = filename.split("/").pop() || filename;

  return basename
    .replace(/[^a-zA-Z0-9_\-.]/g, "_") // No forward slashes allowed
    .replace(/\.+/g, ".")
    .replace(/^\./, "")
    .slice(0, 255);
}
