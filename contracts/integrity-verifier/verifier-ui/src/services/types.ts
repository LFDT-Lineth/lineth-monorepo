/**
 * Verifier Service Interface
 *
 * Defines the contract for verification services, supporting both
 * client-side (IndexedDB) and server-side (API routes) implementations.
 */

import type { VerificationSummary } from "@consensys/linea-contract-integrity-verifier";
import type { ParsedConfig, VerificationOptions, FileRef, AdapterType } from "@/types";

// ============================================================================
// Stored Data Types
// ============================================================================

/**
 * A file stored in the service's storage layer.
 */
export interface StoredFile {
  /** Original path as referenced in the config */
  originalPath: string;
  /** Filename for display */
  filename: string;
  /** File content (JSON string) */
  content: string;
  /** File type */
  type: "schema" | "artifact";
  /** File size in bytes */
  size: number;
  /** Upload timestamp */
  uploadedAt: string;
}

/**
 * Stored configuration file with parsed metadata.
 */
export interface StoredConfig {
  /** Original filename */
  filename: string;
  /** Raw file content (for re-parsing with env vars) */
  content: string;
  /** Parsed configuration metadata */
  parsed: ParsedConfig;
}

/**
 * A session stored in the service's storage layer.
 */
export interface StoredSession {
  /** Unique session identifier */
  id: string;
  /** Session creation timestamp */
  createdAt: string;
  /** Configuration file (if uploaded) */
  config: StoredConfig | null;
  /** Map of original path to stored file */
  files: Record<string, StoredFile>;
  /** Environment variable values */
  envVarValues: Record<string, string>;
}

// ============================================================================
// Service Interface
// ============================================================================

/**
 * Verification service interface.
 *
 * Implementations can store data client-side (IndexedDB) or server-side (API routes).
 * The interface abstracts storage and verification execution details.
 */
export interface VerifierService {
  /**
   * Returns the storage mode identifier.
   */
  readonly mode: "client" | "server";

  // ---- Session Management ----

  /**
   * Creates a new session.
   * @returns Session ID
   */
  createSession(): Promise<string>;

  /**
   * Retrieves an existing session.
   * @param sessionId - Session ID to retrieve
   * @returns Session data or null if not found/expired
   */
  getSession(sessionId: string): Promise<StoredSession | null>;

  /**
   * Deletes a session and all associated data.
   * @param sessionId - Session ID to delete
   */
  deleteSession(sessionId: string): Promise<void>;

  // ---- Configuration ----

  /**
   * Saves and parses a configuration file.
   * @param sessionId - Session ID
   * @param file - Configuration file
   * @returns Parsed configuration metadata
   */
  saveConfig(sessionId: string, file: File): Promise<ParsedConfig>;

  // ---- Files ----

  /**
   * Saves a schema or artifact file.
   * @param sessionId - Session ID
   * @param file - File to save
   * @param type - File type (schema or artifact)
   * @param originalPath - Original path as referenced in config
   */
  saveFile(sessionId: string, file: File, type: "schema" | "artifact", originalPath: string): Promise<void>;

  /**
   * Retrieves a stored file.
   * @param sessionId - Session ID
   * @param originalPath - Original path as referenced in config
   * @returns Stored file or null if not found
   */
  getFile(sessionId: string, originalPath: string): Promise<StoredFile | null>;

  /**
   * Gets all required files and their upload status.
   * @param sessionId - Session ID
   * @returns Array of file references with upload status
   */
  getRequiredFiles(sessionId: string): Promise<FileRef[]>;

  // ---- Verification ----

  /**
   * Runs verification for all contracts in the session.
   * @param sessionId - Session ID
   * @param adapter - Web3 adapter type (viem or ethers)
   * @param envVars - Environment variable values
   * @param options - Verification options
   * @returns Verification summary with results
   */
  runVerification(
    sessionId: string,
    adapter: AdapterType,
    envVars: Record<string, string>,
    options: Omit<VerificationOptions, "adapter">,
  ): Promise<VerificationSummary>;
}

// ============================================================================
// Service Error
// ============================================================================

/**
 * Error thrown by verifier services.
 */
export class VerifierServiceError extends Error {
  constructor(
    public readonly code: string,
    message: string,
    public readonly details?: unknown,
  ) {
    super(message);
    this.name = "VerifierServiceError";
  }
}

/**
 * Error codes for verifier service errors.
 */
export const ServiceErrorCodes = {
  SESSION_NOT_FOUND: "SESSION_NOT_FOUND",
  SESSION_EXPIRED: "SESSION_EXPIRED",
  CONFIG_NOT_FOUND: "CONFIG_NOT_FOUND",
  FILE_NOT_FOUND: "FILE_NOT_FOUND",
  PARSE_ERROR: "PARSE_ERROR",
  VALIDATION_ERROR: "VALIDATION_ERROR",
  STORAGE_ERROR: "STORAGE_ERROR",
  VERIFICATION_FAILED: "VERIFICATION_FAILED",
  NOT_IMPLEMENTED: "NOT_IMPLEMENTED",
} as const;
