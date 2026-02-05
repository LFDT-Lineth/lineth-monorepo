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

  async getSession(_sessionId: string): Promise<StoredSession | null> {
    this.throwNotImplemented();
  }

  async deleteSession(_sessionId: string): Promise<void> {
    this.throwNotImplemented();
  }

  async saveConfig(_sessionId: string, _file: File): Promise<ParsedConfig> {
    this.throwNotImplemented();
  }

  async saveFile(_sessionId: string, _file: File, _type: "schema" | "artifact", _originalPath: string): Promise<void> {
    this.throwNotImplemented();
  }

  async getFile(_sessionId: string, _originalPath: string): Promise<StoredFile | null> {
    this.throwNotImplemented();
  }

  async getRequiredFiles(_sessionId: string): Promise<FileRef[]> {
    this.throwNotImplemented();
  }

  async runVerification(
    _sessionId: string,
    _adapter: AdapterType,
    _envVars: Record<string, string>,
    _options: Omit<VerificationOptions, "adapter">,
  ): Promise<VerificationSummary> {
    this.throwNotImplemented();
  }
}
