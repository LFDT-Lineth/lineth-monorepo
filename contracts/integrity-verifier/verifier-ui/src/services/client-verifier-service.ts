/**
 * Client Verifier Service
 *
 * Browser-based implementation of the VerifierService interface.
 * Uses IndexedDB for storage and runs verification directly in the browser.
 */

// Import from /browser entry point to avoid Node.js 'fs' module
import {
  Verifier,
  parseArtifact,
  parseStorageSchema,
  type VerificationSummary,
  type ContractVerificationResult,
  type VerifierConfig,
  type NormalizedArtifact,
  type StorageSchema,
} from "@consensys/linea-contract-integrity-verifier/browser";
import { ViemAdapter } from "@consensys/linea-contract-integrity-verifier-viem";
import type { ParsedConfig, VerificationOptions, FileRef, AdapterType } from "@/types";
import type { VerifierService, StoredSession, StoredFile } from "./types";
import { VerifierServiceError, ServiceErrorCodes } from "./types";
import {
  saveSession as dbSaveSession,
  getSession as dbGetSession,
  deleteSession as dbDeleteSession,
  generateUUID,
  isIndexedDBAvailable,
} from "@/lib/indexed-db";
import { parseConfig, interpolateEnvVarsInContent, parseMarkdownConfig } from "@/lib/config-parser";
import { DEFAULT_VERIFICATION_TIMEOUT_MS } from "@/lib/constants";

// ============================================================================
// Constants
// ============================================================================

const ALLOWED_CONFIG_EXTENSIONS = [".json", ".md"];
const ALLOWED_FILE_EXTENSIONS = [".json"];
const MAX_FILE_SIZE = 10 * 1024 * 1024; // 10MB

/** Timeout for individual contract verification in milliseconds */
const VERIFICATION_TIMEOUT_MS = DEFAULT_VERIFICATION_TIMEOUT_MS;

// ============================================================================
// Timeout Helper
// ============================================================================

/**
 * Wraps a promise with a timeout. Rejects with a timeout error if the promise
 * doesn't resolve within the specified time.
 */
function withTimeout<T>(promise: Promise<T>, timeoutMs: number, errorMessage: string): Promise<T> {
  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => {
      reject(new Error(errorMessage));
    }, timeoutMs);

    promise
      .then((result) => {
        clearTimeout(timer);
        resolve(result);
      })
      .catch((error) => {
        clearTimeout(timer);
        reject(error);
      });
  });
}

// ============================================================================
// Session Locking
// ============================================================================

/**
 * Simple per-session lock to prevent race conditions on concurrent saves.
 */
const sessionLocks = new Map<string, Promise<void>>();

/**
 * Acquires a lock for a session, waiting for any pending operation to complete.
 * Returns a release function that must be called when the operation is done.
 */
async function acquireSessionLock(sessionId: string): Promise<() => void> {
  // Wait for any existing lock to release
  const existingLock = sessionLocks.get(sessionId);
  if (existingLock) {
    await existingLock.catch(() => {
      /* ignore errors from previous operations */
    });
  }

  // Create a new lock with a resolve function we'll return
  let releaseLock: () => void = () => {};
  const lockPromise = new Promise<void>((resolve) => {
    releaseLock = resolve;
  });

  sessionLocks.set(sessionId, lockPromise);

  return () => {
    releaseLock();
    // Clean up after a short delay to allow any queued operations to see the lock
    setTimeout(() => {
      if (sessionLocks.get(sessionId) === lockPromise) {
        sessionLocks.delete(sessionId);
      }
    }, 10);
  };
}

// ============================================================================
// Client Verifier Service
// ============================================================================

/**
 * Client-side verifier service using IndexedDB for storage.
 * Runs verification directly in the browser using viem adapter.
 */
export class ClientVerifierService implements VerifierService {
  readonly mode = "client" as const;

  constructor() {
    // Check IndexedDB availability
    if (typeof window !== "undefined" && !isIndexedDBAvailable()) {
      console.warn("IndexedDB is not available. Session persistence will not work.");
    }
  }

  // ---- Session Management ----

  async createSession(): Promise<string> {
    const sessionId = generateUUID();
    const now = new Date().toISOString();

    const session: StoredSession = {
      id: sessionId,
      createdAt: now,
      config: null,
      files: {},
      envVarValues: {},
    };

    try {
      await dbSaveSession(session);
    } catch (error) {
      throw new VerifierServiceError(
        ServiceErrorCodes.STORAGE_ERROR,
        `Failed to create session: ${error instanceof Error ? error.message : "Unknown error"}`,
      );
    }

    return sessionId;
  }

  async getSession(sessionId: string): Promise<StoredSession | null> {
    try {
      return await dbGetSession(sessionId);
    } catch (error) {
      throw new VerifierServiceError(
        ServiceErrorCodes.STORAGE_ERROR,
        `Failed to get session: ${error instanceof Error ? error.message : "Unknown error"}`,
      );
    }
  }

  async deleteSession(sessionId: string): Promise<void> {
    try {
      await dbDeleteSession(sessionId);
    } catch (error) {
      throw new VerifierServiceError(
        ServiceErrorCodes.STORAGE_ERROR,
        `Failed to delete session: ${error instanceof Error ? error.message : "Unknown error"}`,
      );
    }
  }

  // ---- Configuration ----

  async saveConfig(sessionId: string, file: File): Promise<ParsedConfig> {
    // Validate file
    this.validateFile(file, ALLOWED_CONFIG_EXTENSIONS);

    // Read file content
    const content = await file.text();

    // Parse config
    let parsed: ParsedConfig;
    try {
      parsed = parseConfig(content, file.name);
    } catch (error) {
      throw new VerifierServiceError(
        ServiceErrorCodes.PARSE_ERROR,
        `Failed to parse config: ${error instanceof Error ? error.message : "Unknown error"}`,
      );
    }

    // Get and update session
    const session = await this.getSessionOrThrow(sessionId);
    session.config = {
      filename: file.name,
      content,
      parsed,
    };
    // Clear files when config changes
    session.files = {};
    session.envVarValues = {};

    await this.saveSessionOrThrow(session);

    return parsed;
  }

  // ---- Files ----

  async saveFile(sessionId: string, file: File, type: "schema" | "artifact", originalPath: string): Promise<void> {
    // Validate file
    this.validateFile(file, ALLOWED_FILE_EXTENSIONS);

    // Validate JSON content
    const content = await file.text();
    try {
      JSON.parse(content);
    } catch {
      throw new VerifierServiceError(ServiceErrorCodes.PARSE_ERROR, "Invalid JSON file");
    }

    // Acquire lock to prevent race conditions on concurrent file saves
    const releaseLock = await acquireSessionLock(sessionId);
    try {
      // Get and update session
      const session = await this.getSessionOrThrow(sessionId);

      session.files[originalPath] = {
        originalPath,
        filename: file.name,
        content,
        type,
        size: file.size,
        uploadedAt: new Date().toISOString(),
      };

      await this.saveSessionOrThrow(session);
    } finally {
      releaseLock();
    }
  }

  async getFile(sessionId: string, originalPath: string): Promise<StoredFile | null> {
    const session = await this.getSessionOrThrow(sessionId);
    return session.files[originalPath] ?? null;
  }

  async getRequiredFiles(sessionId: string): Promise<FileRef[]> {
    const session = await this.getSessionOrThrow(sessionId);

    if (!session.config) {
      return [];
    }

    // Update uploaded status based on stored files
    return session.config.parsed.requiredFiles.map((file) => ({
      ...file,
      uploaded: session.files[file.path] !== undefined,
    }));
  }

  // ---- Verification ----

  async runVerification(
    sessionId: string,
    adapter: AdapterType,
    envVars: Record<string, string>,
    options: Omit<VerificationOptions, "adapter">,
  ): Promise<VerificationSummary> {
    const session = await this.getSessionOrThrow(sessionId);

    if (!session.config) {
      throw new VerifierServiceError(ServiceErrorCodes.CONFIG_NOT_FOUND, "No configuration uploaded");
    }

    // Check all required files are uploaded
    const missingFiles = session.config.parsed.requiredFiles.filter((f) => !session.files[f.path]);
    if (missingFiles.length > 0) {
      throw new VerifierServiceError(
        ServiceErrorCodes.FILE_NOT_FOUND,
        `Missing required files: ${missingFiles.map((f) => f.path).join(", ")}`,
      );
    }

    // Check all env vars are provided
    const missingEnvVars = session.config.parsed.envVars.filter((v) => !envVars[v] || envVars[v].trim() === "");
    if (missingEnvVars.length > 0) {
      throw new VerifierServiceError(
        ServiceErrorCodes.VALIDATION_ERROR,
        `Missing environment variables: ${missingEnvVars.join(", ")}`,
      );
    }

    // Interpolate env vars and parse config
    let config: VerifierConfig;
    try {
      const interpolatedContent = interpolateEnvVarsInContent(session.config.content, envVars);

      if (session.config.parsed.format === "markdown") {
        config = parseMarkdownConfig(interpolatedContent);
      } else {
        config = JSON.parse(interpolatedContent) as VerifierConfig;
      }
    } catch (error) {
      throw new VerifierServiceError(
        ServiceErrorCodes.PARSE_ERROR,
        `Failed to parse config: ${error instanceof Error ? error.message : "Unknown error"}`,
      );
    }

    // Load artifacts and schemas from stored files
    const artifacts = new Map<string, NormalizedArtifact>();
    const schemas = new Map<string, StorageSchema>();

    for (const [path, storedFile] of Object.entries(session.files)) {
      try {
        if (storedFile.type === "artifact") {
          artifacts.set(path, parseArtifact(storedFile.content, storedFile.filename));
        } else if (storedFile.type === "schema") {
          schemas.set(path, parseStorageSchema(storedFile.content));
        }
      } catch (error) {
        throw new VerifierServiceError(
          ServiceErrorCodes.PARSE_ERROR,
          `Failed to parse ${storedFile.type} file ${path}: ${error instanceof Error ? error.message : "Unknown error"}`,
        );
      }
    }

    // Create adapters for each chain (only viem supported in browser)
    if (adapter !== "viem") {
      console.warn(`Adapter "${adapter}" is not supported in browser mode. Using viem instead.`);
    }

    const adapters = new Map<string, InstanceType<typeof ViemAdapter>>();
    for (const [chainName, chainConfig] of Object.entries(config.chains)) {
      if (chainConfig.rpcUrl) {
        adapters.set(chainName, new ViemAdapter({ rpcUrl: chainConfig.rpcUrl, chainId: chainConfig.chainId }));
      }
    }

    // Filter contracts if specified
    let contractsToVerify = config.contracts;
    if (options.contractFilter) {
      contractsToVerify = contractsToVerify.filter(
        (c) => c.name.toLowerCase() === options.contractFilter!.toLowerCase(),
      );
    }
    if (options.chainFilter) {
      contractsToVerify = contractsToVerify.filter((c) => c.chain.toLowerCase() === options.chainFilter!.toLowerCase());
    }

    // Run verification for each contract
    const results: ContractVerificationResult[] = [];
    let passed = 0;
    let failed = 0;
    let warnings = 0;
    let skipped = 0;

    for (const contract of contractsToVerify) {
      const chainAdapter = adapters.get(contract.chain);
      if (!chainAdapter) {
        skipped++;
        continue;
      }

      const chain = config.chains[contract.chain];
      const verifier = new Verifier(chainAdapter);

      // Get artifact and schema for this contract
      const artifact = artifacts.get(contract.artifactFile);
      if (!artifact) {
        results.push({
          contract,
          chain,
          error: `Artifact not found: ${contract.artifactFile}`,
        });
        failed++;
        continue;
      }

      const schema = contract.stateVerification?.schemaFile
        ? schemas.get(contract.stateVerification.schemaFile)
        : undefined;

      try {
        const result = await withTimeout(
          verifier.verifyContractWithContent(
            contract,
            chain,
            {
              verbose: options.verbose,
              skipBytecode: options.skipBytecode,
              skipAbi: options.skipAbi,
              skipState: options.skipState,
            },
            { artifact, schema },
          ),
          VERIFICATION_TIMEOUT_MS,
          `Verification timeout for ${contract.name} (exceeded ${VERIFICATION_TIMEOUT_MS / 1000}s)`,
        );

        results.push(result);

        // Count results
        if (result.error) {
          failed++;
        } else {
          const bytecodeStatus = result.bytecodeResult?.status;
          const abiStatus = result.abiResult?.status;
          const stateStatus = result.stateResult?.status;

          const statuses = [bytecodeStatus, abiStatus, stateStatus].filter(Boolean);

          if (bytecodeStatus === "fail" || abiStatus === "fail" || stateStatus === "fail") {
            failed++;
          } else if (bytecodeStatus === "warn" || abiStatus === "warn" || stateStatus === "warn") {
            warnings++;
          } else if (statuses.length > 0 && statuses.every((s) => s === "skip")) {
            skipped++;
          } else {
            passed++;
          }
        }
      } catch (error) {
        results.push({
          contract,
          chain,
          error: error instanceof Error ? error.message : "Verification failed",
        });
        failed++;
      }
    }

    return {
      total: contractsToVerify.length,
      passed,
      failed,
      warnings,
      skipped,
      results,
    };
  }

  // ---- Private Helpers ----

  private async getSessionOrThrow(sessionId: string): Promise<StoredSession> {
    const session = await this.getSession(sessionId);
    if (!session) {
      throw new VerifierServiceError(ServiceErrorCodes.SESSION_NOT_FOUND, "Session not found");
    }
    return session;
  }

  private async saveSessionOrThrow(session: StoredSession): Promise<void> {
    try {
      await dbSaveSession(session);
    } catch (error) {
      throw new VerifierServiceError(
        ServiceErrorCodes.STORAGE_ERROR,
        `Failed to save session: ${error instanceof Error ? error.message : "Unknown error"}`,
      );
    }
  }

  private validateFile(file: File, allowedExtensions: string[]): void {
    // Check file size
    if (file.size > MAX_FILE_SIZE) {
      throw new VerifierServiceError(
        ServiceErrorCodes.VALIDATION_ERROR,
        `File too large. Maximum size is ${MAX_FILE_SIZE / 1024 / 1024}MB`,
      );
    }

    // Check file extension
    const ext = "." + file.name.split(".").pop()?.toLowerCase();
    if (!allowedExtensions.includes(ext)) {
      throw new VerifierServiceError(
        ServiceErrorCodes.VALIDATION_ERROR,
        `Invalid file extension. Allowed: ${allowedExtensions.join(", ")}`,
      );
    }
  }
}
