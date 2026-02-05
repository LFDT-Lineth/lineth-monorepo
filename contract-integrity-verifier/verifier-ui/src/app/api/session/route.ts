/**
 * Session Management API Routes
 *
 * NOTE: SERVER MODE ONLY
 * These routes are only used when NEXT_PUBLIC_STORAGE_MODE=server.
 * In client mode (default), sessions are managed via IndexedDB in the browser.
 *
 * To enable server mode in the future, implement ServerVerifierService in
 * src/services/server-verifier-service.ts to use these API routes.
 */

// Required for static export - makes these routes return 404 in static builds
// eslint-disable-next-line @typescript-eslint/no-unused-vars
export const dynamic = "force-static";

import { NextResponse } from "next/server";
import { createSession, getSession } from "@/lib/session";
import type { ApiError, SessionResponse } from "@/types";

// POST /api/session - Create new session
export async function POST(): Promise<NextResponse<SessionResponse | ApiError>> {
  try {
    const session = await createSession();
    return NextResponse.json({ sessionId: session.id });
  } catch (error) {
    console.error("Failed to create session:", error);
    return NextResponse.json({ code: "INTERNAL_ERROR", message: "Failed to create session" }, { status: 500 });
  }
}

// GET /api/session?id=xxx - Get session info
export async function GET(request: Request): Promise<NextResponse<SessionResponse | ApiError>> {
  const { searchParams } = new URL(request.url);
  const sessionId = searchParams.get("id");

  if (!sessionId) {
    return NextResponse.json({ code: "SESSION_NOT_FOUND", message: "Session ID required" }, { status: 400 });
  }

  try {
    const session = await getSession(sessionId);

    if (!session) {
      return NextResponse.json({ code: "SESSION_EXPIRED", message: "Session not found or expired" }, { status: 404 });
    }

    return NextResponse.json({ sessionId: session.id });
  } catch (error) {
    console.error("Failed to get session:", error);
    return NextResponse.json({ code: "INTERNAL_ERROR", message: "Failed to get session" }, { status: 500 });
  }
}
