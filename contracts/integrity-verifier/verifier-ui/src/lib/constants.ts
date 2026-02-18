/**
 * Constants for the verifier UI.
 *
 * NOTE: This file is imported in browser contexts. Do NOT add Node.js-only
 * imports (os, fs, path, etc.) here. Server-only constants are in constants-server.ts.
 */

// Session configuration (browser-safe)
export const SESSION_EXPIRY_HOURS = 24;
export const SESSION_EXPIRY_MS = SESSION_EXPIRY_HOURS * 60 * 60 * 1000;

// File limits
export const MAX_FILE_SIZE = 10 * 1024 * 1024; // 10MB

// Allowed file types
export const ALLOWED_CONFIG_EXTENSIONS = [".json", ".md"];
export const ALLOWED_SCHEMA_EXTENSIONS = [".json"];
export const ALLOWED_ARTIFACT_EXTENSIONS = [".json"];

// API paths
export const API_SESSION = "/api/session";
export const API_UPLOAD = "/api/upload";
export const API_VERIFY = "/api/verify";

// Verification timeouts (in milliseconds)
// Can be overridden via environment variable VERIFICATION_TIMEOUT_MS
export const DEFAULT_VERIFICATION_TIMEOUT_MS = 60 * 1000; // 60 seconds

// Input validation limits
export const MAX_ENV_VAR_NAME_LENGTH = 256;
export const MAX_ENV_VAR_VALUE_LENGTH = 4096;
