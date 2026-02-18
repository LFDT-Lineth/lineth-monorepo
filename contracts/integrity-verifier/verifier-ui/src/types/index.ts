import type {
  VerifierConfig,
  ContractConfig,
  ChainConfig,
  ContractVerificationResult,
  VerificationSummary,
} from "@consensys/linea-contract-integrity-verifier";

// Re-export core types
export type { VerifierConfig, ContractConfig, ChainConfig, ContractVerificationResult, VerificationSummary };

// ============================================================================
// Adapter Types
// ============================================================================

export type AdapterType = "ethers" | "viem";

// ============================================================================
// Session Types
// ============================================================================

export interface Session {
  id: string;
  createdAt: string;
  expiresAt: string;
  config: ParsedConfig | null;
  fileMap: Record<string, string>;
  envVarValues: Record<string, string>;
}

// ============================================================================
// Config Parsing Types
// ============================================================================

export type ConfigFormat = "json" | "markdown";

export interface FileRef {
  /** Original path as specified in config */
  path: string;
  /** Type of file */
  type: "schema" | "artifact";
  /** Contract name that requires this file */
  contractName: string;
  /** Whether file has been uploaded */
  uploaded: boolean;
}

export interface ParsedConfig {
  /** Raw config object (with placeholders for env vars) */
  raw: VerifierConfig;
  /** Original raw content string for later interpolation */
  rawContent: string;
  /** Original filename */
  filename: string;
  /** Config format */
  format: ConfigFormat;
  /** Extracted environment variable names */
  envVars: string[];
  /** Required schema and artifact files */
  requiredFiles: FileRef[];
  /** Chain names defined in config */
  chains: string[];
  /** Contract names defined in config */
  contracts: string[];
}

// ============================================================================
// Verification Options
// ============================================================================

export interface VerificationOptions {
  adapter: AdapterType;
  verbose: boolean;
  skipBytecode: boolean;
  skipAbi: boolean;
  skipState: boolean;
  contractFilter?: string;
  chainFilter?: string;
}

export const defaultVerificationOptions: VerificationOptions = {
  adapter: "viem",
  verbose: false,
  skipBytecode: false,
  skipAbi: false,
  skipState: false,
};

// ============================================================================
// Form Field Types
// ============================================================================

export type FieldType = "text" | "url" | "address" | "number" | "password";

export interface FormField {
  /** Environment variable name */
  name: string;
  /** Input type */
  type: FieldType;
  /** Display label */
  label: string;
  /** Placeholder text */
  placeholder?: string;
  /** Whether field is required */
  required: boolean;
}

// ============================================================================
// UI State Types
// ============================================================================

export type UploadStatus = "idle" | "uploading" | "success" | "error";
export type VerifyStatus = "idle" | "running" | "complete" | "error";

export interface UploadedFile {
  originalPath: string;
  uploadedPath: string;
  filename: string;
  size: number;
  status: UploadStatus;
  error?: string;
}

// ============================================================================
// API Types
// ============================================================================

export interface ApiError {
  code: string;
  message: string;
  details?: unknown;
}

export interface SessionResponse {
  sessionId: string;
}

export interface UploadResponse {
  sessionId: string;
  uploadedPath: string;
  parsedConfig?: ParsedConfig;
}

export interface VerifyRequest {
  sessionId: string;
  adapter: AdapterType;
  envVars: Record<string, string>;
  options: Omit<VerificationOptions, "adapter">;
}

export interface VerifyResponse {
  summary: VerificationSummary;
}

// ============================================================================
// Error Types
// ============================================================================

export type VerifierUIErrorCode =
  | "INVALID_CONFIG"
  | "MISSING_FILE"
  | "MISSING_ENV_VAR"
  | "UPLOAD_FAILED"
  | "PARSE_ERROR"
  | "RPC_ERROR"
  | "VERIFICATION_FAILED"
  | "SESSION_EXPIRED"
  | "SESSION_NOT_FOUND"
  | "INTERNAL_ERROR";

export interface VerifierUIError {
  code: VerifierUIErrorCode;
  message: string;
  details?: unknown;
}
