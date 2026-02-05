import type {
  SessionResponse,
  UploadResponse,
  VerifyRequest,
  VerifyResponse,
  ApiError,
  AdapterType,
  VerificationOptions,
} from "@/types";
import { API_SESSION, API_UPLOAD, API_VERIFY } from "./constants";

// ============================================================================
// API Client
// ============================================================================

class ApiClient {
  private baseUrl: string;

  constructor(baseUrl: string = "") {
    this.baseUrl = baseUrl;
  }

  private async request<T>(endpoint: string, options: RequestInit = {}): Promise<T> {
    const url = `${this.baseUrl}${endpoint}`;

    const response = await fetch(url, {
      ...options,
      headers: {
        ...options.headers,
      },
    });

    if (!response.ok) {
      const error = await response.json().catch(() => ({
        code: "INTERNAL_ERROR",
        message: response.statusText,
      }));
      throw new ApiClientError(error as ApiError);
    }

    return response.json() as Promise<T>;
  }

  // Session management
  async createSession(): Promise<SessionResponse> {
    return this.request<SessionResponse>(API_SESSION, {
      method: "POST",
    });
  }

  async getSession(sessionId: string): Promise<SessionResponse> {
    return this.request<SessionResponse>(`${API_SESSION}?id=${sessionId}`, {
      method: "GET",
    });
  }

  // File upload
  async uploadConfig(sessionId: string, file: File): Promise<UploadResponse> {
    const formData = new FormData();
    formData.append("file", file);
    formData.append("sessionId", sessionId);
    formData.append("type", "config");

    return this.request<UploadResponse>(API_UPLOAD, {
      method: "POST",
      body: formData,
    });
  }

  async uploadFile(
    sessionId: string,
    file: File,
    type: "schema" | "artifact",
    originalPath: string,
  ): Promise<UploadResponse> {
    const formData = new FormData();
    formData.append("file", file);
    formData.append("sessionId", sessionId);
    formData.append("type", type);
    formData.append("originalPath", originalPath);

    return this.request<UploadResponse>(API_UPLOAD, {
      method: "POST",
      body: formData,
    });
  }

  // Verification
  async runVerification(
    sessionId: string,
    adapter: AdapterType,
    envVars: Record<string, string>,
    options: Omit<VerificationOptions, "adapter">,
  ): Promise<VerifyResponse> {
    const request: VerifyRequest = {
      sessionId,
      adapter,
      envVars,
      options,
    };

    return this.request<VerifyResponse>(API_VERIFY, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(request),
    });
  }
}

// ============================================================================
// Error Class
// ============================================================================

export class ApiClientError extends Error {
  code: string;
  details?: unknown;

  constructor(error: ApiError) {
    super(error.message);
    this.name = "ApiClientError";
    this.code = error.code;
    this.details = error.details;
  }
}

// ============================================================================
// Singleton Export
// ============================================================================

export const apiClient = new ApiClient();
