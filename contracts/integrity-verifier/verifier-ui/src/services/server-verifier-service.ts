/**
 * Server Verifier Service (Placeholder)
 *
 * This is a placeholder implementation for server-side verification.
 * When server mode is implemented, this service would use the existing
 * API routes in /app/api/ for session management, file storage, and verification.
 *
 * To enable server mode in the future:
 * 1. Implement the methods below using the existing apiClient
 * 2. Set NEXT_PUBLIC_STORAGE_MODE=server in environment
 */

import type { VerificationSummary } from "@consensys/linea-contract-integrity-verifier";
import type { ParsedConfig, VerificationOptions, FileRef, AdapterType } from "@/types";
import type { VerifierService, StoredSession, StoredFile } from "./types";
import { VerifierServiceError, ServiceErrorCodes } from "./types";

/**
 * Server-side verifier service using API routes.
 *
 * NOTE: This is a placeholder implementation. All methods throw "NOT_IMPLEMENTED"
 * errors. The existing API routes in /app/api/ remain available for future use.
 */
export class ServerVerifierService implements VerifierService {
  readonly mode = "server" as const;

  private throwNotImplemented(): never {
    throw new VerifierServiceError(
      ServiceErrorCodes.NOT_IMPLEMENTED,
      "Server mode is not implemented. Set NEXT_PUBLIC_STORAGE_MODE=client to use client-side storage.",
    );
  }

  async createSession(): Promise<string> {
    this.throwNotImplemented();
  }

  async getSession(sessionId: string): Promise<StoredSession | null> {
    void sessionId;
    this.throwNotImplemented();
  }

  async deleteSession(sessionId: string): Promise<void> {
    void sessionId;
    this.throwNotImplemented();
  }

  async saveConfig(sessionId: string, file: File): Promise<ParsedConfig> {
    void sessionId;
    void file;
    this.throwNotImplemented();
  }

  async saveFile(sessionId: string, file: File, type: "schema" | "artifact", originalPath: string): Promise<void> {
    void sessionId;
    void file;
    void type;
    void originalPath;
    this.throwNotImplemented();
  }

  async getFile(sessionId: string, originalPath: string): Promise<StoredFile | null> {
    void sessionId;
    void originalPath;
    this.throwNotImplemented();
  }

  async getRequiredFiles(sessionId: string): Promise<FileRef[]> {
    void sessionId;
    this.throwNotImplemented();
  }

  async runVerification(
    sessionId: string,
    adapter: AdapterType,
    envVars: Record<string, string>,
    options: Omit<VerificationOptions, "adapter">,
  ): Promise<VerificationSummary> {
    void sessionId;
    void adapter;
    void envVars;
    void options;
    this.throwNotImplemented();
  }
}
