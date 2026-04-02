import assert from "node:assert/strict";
import test from "node:test";

import { HARDHAT_SIGNER_UI_SESSION_TOKEN_HEADER, __testOnly, postJson } from "../app/bridgeApiClient.ts";

test("postJson sends JSON payload and signer-ui session header", async () => {
  let capturedUrl: string | undefined;
  let capturedInit: RequestInit | undefined;

  const fetchMock: typeof fetch = async (input, init) => {
    capturedUrl = String(input);
    capturedInit = init;
    return new Response(JSON.stringify({ ok: true }), { status: 200 });
  };

  await postJson(
    "http://127.0.0.1:15555/api/respond",
    { requestId: "abc", hash: "0x1234" },
    "session-secret",
    fetchMock,
  );

  assert.equal(capturedUrl, "http://127.0.0.1:15555/api/respond");
  assert.equal(capturedInit?.method, "POST");
  assert.equal(capturedInit?.body, JSON.stringify({ requestId: "abc", hash: "0x1234" }));

  const headers = capturedInit?.headers as Record<string, string>;
  assert.equal(headers["Content-Type"], "application/json");
  assert.equal(headers[HARDHAT_SIGNER_UI_SESSION_TOKEN_HEADER], "session-secret");
});

test("postJson surfaces JSON { error } body on failure", async () => {
  const fetchMock: typeof fetch = async () => {
    return new Response(JSON.stringify({ error: "chainId does not match this Hardhat signer session." }), {
      status: 400,
      headers: { "Content-Type": "application/json" },
    });
  };

  await assert.rejects(
    () => postJson("http://127.0.0.1:15555/api/respond", { requestId: "abc" }, "session-secret", fetchMock),
    (error: unknown) => {
      assert.equal((error as Error).message, "chainId does not match this Hardhat signer session.");
      return true;
    },
  );
});

test("postJson falls back to plain-text body on failure", async () => {
  const fetchMock: typeof fetch = async () => {
    return new Response("Malformed transaction response", { status: 400 });
  };

  await assert.rejects(
    () => postJson("http://127.0.0.1:15555/api/respond", { requestId: "abc" }, "session-secret", fetchMock),
    (error: unknown) => {
      assert.equal((error as Error).message, "Malformed transaction response");
      return true;
    },
  );
});

test("postJson uses status fallback when failure body is empty", async () => {
  const fetchMock: typeof fetch = async () => {
    return new Response("", { status: 503 });
  };

  await assert.rejects(
    () => postJson("http://127.0.0.1:15555/api/session", {}, "session-secret", fetchMock),
    (error: unknown) => {
      assert.equal((error as Error).message, "Request to http://127.0.0.1:15555/api/session failed with 503");
      return true;
    },
  );
});

test("postJson propagates fetch-level network errors", async () => {
  const fetchMock: typeof fetch = async () => {
    throw new Error("connect ECONNREFUSED 127.0.0.1:15555");
  };

  await assert.rejects(
    () => postJson("http://127.0.0.1:15555/api/session", {}, "session-secret", fetchMock),
    (error: unknown) => {
      assert.equal((error as Error).message, "connect ECONNREFUSED 127.0.0.1:15555");
      return true;
    },
  );
});

test("parseErrorMessageFromBody trims whitespace and handles non-error JSON", () => {
  assert.equal(__testOnly.parseErrorMessageFromBody(""), undefined);
  assert.equal(__testOnly.parseErrorMessageFromBody("   \n\t "), undefined);
  assert.equal(__testOnly.parseErrorMessageFromBody('{"error":"  wallet rejected  "}'), "wallet rejected");
  assert.equal(
    __testOnly.parseErrorMessageFromBody('{"message":"missing error key"}'),
    '{"message":"missing error key"}',
  );
  assert.equal(__testOnly.parseErrorMessageFromBody("  plain text failure  "), "plain text failure");
});
