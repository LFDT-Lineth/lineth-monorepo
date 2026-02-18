/**
 * File Upload API Routes
 *
 * NOTE: SERVER MODE ONLY
 * These routes are only used when NEXT_PUBLIC_STORAGE_MODE=server.
 * In client mode (default), files are stored via IndexedDB in the browser.
 *
 * To enable server mode in the future, implement ServerVerifierService in
 * src/services/server-verifier-service.ts to use these API routes.
 */

// Required for static export - makes these routes return 404 in static builds

export const dynamic = "force-static";

import { NextResponse } from "next/server";
import { getSession, updateSession, storeConfigFile, storeFile } from "@/lib/session";
import { parseConfig } from "@/lib/config-parser";
import {
  MAX_FILE_SIZE,
  ALLOWED_CONFIG_EXTENSIONS,
  ALLOWED_SCHEMA_EXTENSIONS,
  ALLOWED_ARTIFACT_EXTENSIONS,
} from "@/lib/constants";
import { sanitizeFilename } from "@/lib/validation";
import type { ApiError, UploadResponse } from "@/types";

// POST /api/upload - Upload file
export async function POST(request: Request): Promise<NextResponse<UploadResponse | ApiError>> {
  try {
    const formData = await request.formData();

    const file = formData.get("file") as File | null;
    const sessionId = formData.get("sessionId") as string | null;
    const type = formData.get("type") as "config" | "schema" | "artifact" | null;
    const originalPath = formData.get("originalPath") as string | null;

    // Validate required fields
    if (!file) {
      return NextResponse.json({ code: "UPLOAD_FAILED", message: "No file provided" }, { status: 400 });
    }

    if (!sessionId) {
      return NextResponse.json({ code: "UPLOAD_FAILED", message: "Session ID required" }, { status: 400 });
    }

    if (!type || !["config", "schema", "artifact"].includes(type)) {
      return NextResponse.json({ code: "UPLOAD_FAILED", message: "Invalid file type" }, { status: 400 });
    }

    // Validate session
    const session = await getSession(sessionId);
    if (!session) {
      return NextResponse.json({ code: "SESSION_EXPIRED", message: "Session not found or expired" }, { status: 404 });
    }

    // Validate file size
    if (file.size > MAX_FILE_SIZE) {
      return NextResponse.json(
        {
          code: "UPLOAD_FAILED",
          message: `File too large. Maximum size is ${MAX_FILE_SIZE / 1024 / 1024}MB`,
        },
        { status: 400 },
      );
    }

    // Validate file extension
    const filename = sanitizeFilename(file.name);
    const ext = `.${filename.split(".").pop()?.toLowerCase()}`;

    const allowedExtensions =
      type === "config"
        ? ALLOWED_CONFIG_EXTENSIONS
        : type === "schema"
          ? ALLOWED_SCHEMA_EXTENSIONS
          : ALLOWED_ARTIFACT_EXTENSIONS;

    if (!allowedExtensions.includes(ext)) {
      return NextResponse.json(
        {
          code: "UPLOAD_FAILED",
          message: `Invalid file extension. Allowed: ${allowedExtensions.join(", ")}`,
        },
        { status: 400 },
      );
    }

    // Read file content
    const content = await file.text();

    // Handle config file
    if (type === "config") {
      try {
        const parsedConfig = parseConfig(content, filename);
        const uploadedPath = await storeConfigFile(sessionId, filename, content);

        await updateSession(sessionId, {
          config: parsedConfig,
          fileMap: {},
          envVarValues: {},
        });

        return NextResponse.json({
          sessionId,
          uploadedPath,
          parsedConfig,
        });
      } catch (error) {
        const message = error instanceof Error ? error.message : "Failed to parse config";
        return NextResponse.json({ code: "PARSE_ERROR", message }, { status: 400 });
      }
    }

    // Handle schema/artifact file
    if (!originalPath) {
      return NextResponse.json(
        { code: "UPLOAD_FAILED", message: "Original path required for schema/artifact files" },
        { status: 400 },
      );
    }

    // Validate JSON
    try {
      JSON.parse(content);
    } catch {
      return NextResponse.json({ code: "PARSE_ERROR", message: "Invalid JSON file" }, { status: 400 });
    }

    const uploadedPath = await storeFile(sessionId, type, originalPath, content);

    // Update file map
    const fileMap = { ...session.fileMap, [originalPath]: uploadedPath };
    await updateSession(sessionId, { fileMap });

    return NextResponse.json({
      sessionId,
      uploadedPath,
    });
  } catch (error) {
    console.error("Upload failed:", error);
    return NextResponse.json({ code: "INTERNAL_ERROR", message: "Upload failed" }, { status: 500 });
  }
}
