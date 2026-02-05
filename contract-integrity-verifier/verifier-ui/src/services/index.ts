/**
 * Verifier Service Factory
 *
 * Creates the appropriate verifier service based on configuration.
 * Defaults to client-side storage (IndexedDB) for browser-only operation.
 *
 * Storage mode is controlled via NEXT_PUBLIC_STORAGE_MODE environment variable:
 * - "client" (default): Uses IndexedDB for storage, runs verification in browser
 * - "server": Uses API routes for storage and verification (placeholder, not implemented)
 */

import type { VerifierService } from "./types";

// Re-export types
export type { VerifierService, StoredSession, StoredFile, StoredConfig } from "./types";
export { VerifierServiceError, ServiceErrorCodes } from "./types";

/**
 * Storage mode for the verifier service.
 */
export type StorageMode = "client" | "server";

/**
 * Gets the configured storage mode from environment.
 * Defaults to "client" if not specified.
 */
export function getStorageMode(): StorageMode {
  const mode = process.env.NEXT_PUBLIC_STORAGE_MODE;
  if (mode === "server") {
    return "server";
  }
  return "client";
}

/**
 * Creates a verifier service instance based on the configured storage mode.
 *
 * Note: This function is async to support lazy loading of service implementations.
 */
export async function createVerifierService(): Promise<VerifierService> {
  const mode = getStorageMode();

  if (mode === "server") {
    const { ServerVerifierService } = await import("./server-verifier-service");
    return new ServerVerifierService();
  }

  const { ClientVerifierService } = await import("./client-verifier-service");
  return new ClientVerifierService();
}

// Singleton instance (lazily initialized)
let serviceInstance: VerifierService | null = null;
let servicePromise: Promise<VerifierService> | null = null;

/**
 * Gets the singleton verifier service instance.
 * Lazily initializes the service on first call.
 */
export async function getVerifierService(): Promise<VerifierService> {
  if (serviceInstance) {
    return serviceInstance;
  }

  if (!servicePromise) {
    servicePromise = createVerifierService().then((service) => {
      serviceInstance = service;
      return service;
    });
  }

  return servicePromise;
}

/**
 * Resets the service instance. Useful for testing.
 */
export function resetVerifierService(): void {
  serviceInstance = null;
  servicePromise = null;
}
