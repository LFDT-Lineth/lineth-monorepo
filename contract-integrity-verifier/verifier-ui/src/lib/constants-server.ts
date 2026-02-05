/**
 * Server-only constants for the verifier UI.
 *
 * NOTE: This file uses Node.js modules and should ONLY be imported in
 * server-side code (API routes, session.ts, etc.). Never import this
 * in browser/client code.
 */

import { homedir } from "os";
import { join } from "path";

// Re-export browser-safe constants for convenience
export { SESSION_EXPIRY_MS } from "./constants";

// File storage (server-only)
export const DEFAULT_UPLOADS_DIR = join(homedir(), ".linea-verifier-ui");
export const UPLOADS_DIR = process.env.VERIFIER_UPLOADS_DIR || DEFAULT_UPLOADS_DIR;
export const SESSIONS_DIR = join(UPLOADS_DIR, "sessions");
