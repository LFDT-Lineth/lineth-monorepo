import { Writable } from "node:stream";
import { format, transports } from "winston";

import { WinstonLogger, WinstonLoggerOptions } from "../WinstonLogger";

// eslint-disable-next-line no-control-regex
const stripAnsi = (str: string) => str.replace(/\x1B\[[0-9;]*m/g, "");

describe("WinstonLogger", () => {
  let buffer: string;

  beforeEach(() => {
    buffer = "";
  });

  // Build a logger that writes into an in-memory `Writable` via `transports.Stream`.
  // Avoids monkey-patching `process.stdout.write` (which is brittle across Node versions
  // and bleeds across concurrent test files).
  function makeLogger(name: string, opts: WinstonLoggerOptions = {}): WinstonLogger {
    const stream = new Writable({
      write(chunk, _encoding, callback) {
        buffer += chunk.toString();
        callback();
      },
    });
    return new WinstonLogger(name, {
      ...opts,
      transports: [new transports.Stream({ stream })],
    });
  }

  function getLogOutput(): string {
    return stripAnsi(buffer);
  }

  function getRawOutput(): string {
    return buffer;
  }

  describe("log line structure", () => {
    it("contains time, level, logger, and msg fields", () => {
      const logger = makeLogger("MyService");
      logger.info("hello world");

      const output = getLogOutput();
      expect(output).toMatch(/time=\S+/);
      expect(output).toContain("level=INFO");
      expect(output).toContain("logger=MyService");
      expect(output).toContain('msg="hello world"');
    });

    it("is a single line even when metadata contains newlines", () => {
      const logger = makeLogger("Test");
      logger.info("msg", { reason: "line1\nline2" });

      const lines = getLogOutput()
        .trim()
        .split("\n")
        .filter((l) => l.includes("level="));
      expect(lines).toHaveLength(1);
    });

    it("uppercases the level", () => {
      makeLogger("Test").warn("watch out");
      expect(getLogOutput()).toContain("level=WARN");
    });

    it("only colorizes the level value, not the whole line", () => {
      makeLogger("Test").info("msg");
      const raw = getRawOutput();
      // eslint-disable-next-line no-control-regex
      const ansiLevel = /level=\x1B\[\d+mINFO\x1B\[\d+m/;
      expect(raw).toMatch(ansiLevel);
      // eslint-disable-next-line no-control-regex
      expect(raw.replace(ansiLevel, "")).not.toMatch(/\x1B\[/);
    });
  });

  describe("log levels", () => {
    it("logs at info level", () => {
      makeLogger("Test").info("info msg");
      expect(getLogOutput()).toContain("level=INFO");
    });

    it("logs at warn level", () => {
      makeLogger("Test").warn("warn msg");
      expect(getLogOutput()).toContain("level=WARN");
    });

    it("logs at error level", () => {
      makeLogger("Test").error("error msg");
      expect(getLogOutput()).toContain("level=ERROR");
    });

    it("does not log debug by default", () => {
      makeLogger("Test").debug("debug msg");
      expect(getLogOutput()).toBe("");
    });

    it("logs debug when level option is debug", () => {
      makeLogger("Test", { level: "debug" }).debug("debug msg");
      expect(getLogOutput()).toContain("level=DEBUG");
    });
  });

  describe("logfmt value quoting", () => {
    it("quotes values that contain spaces", () => {
      makeLogger("Test").info("msg", { reason: "something went wrong" });
      expect(getLogOutput()).toContain('reason="something went wrong"');
    });

    it("does not quote simple values", () => {
      makeLogger("Test").info("msg", { foo: "bar" });
      expect(getLogOutput()).toContain("foo=bar");
    });

    it("escapes newlines inside values to keep the entry on one line", () => {
      makeLogger("Test").info("msg", { reason: "line1\nline2" });
      expect(getLogOutput()).toContain('reason="line1\\nline2"');
    });

    it("escapes double-quotes inside values", () => {
      makeLogger("Test").info("msg", { note: 'say "hello"' });
      expect(getLogOutput()).toContain('note="say \\"hello\\""');
    });

    it("escapes backslashes inside values", () => {
      makeLogger("Test").info("msg", { path: "C:\\Users\\foo" });
      expect(getLogOutput()).toContain('path="C:\\\\Users\\\\foo"');
    });
  });

  describe("metadata — no metadata", () => {
    it("produces no extra fields when there is no metadata", () => {
      makeLogger("Test").info("plain");
      const output = getLogOutput();
      expect(output.trim()).toMatch(/^time=\S+ level=\S+ logger=\S+ msg=plain$/);
    });
  });

  describe("metadata — scalar values", () => {
    it("includes multiple string fields", () => {
      makeLogger("Test").info("msg", { a: "1", b: "2" });
      const output = getLogOutput();
      expect(output).toContain("a=1");
      expect(output).toContain("b=2");
    });

    it("serializes numbers directly", () => {
      makeLogger("Test").info("msg", { count: 42 });
      expect(getLogOutput()).toContain("count=42");
    });

    it("serializes booleans directly", () => {
      makeLogger("Test").info("msg", { active: false });
      expect(getLogOutput()).toContain("active=false");
    });

    it("serializes bigints without JSON quotes", () => {
      makeLogger("Test").info("msg", { value: BigInt("99999999999999999999") });
      expect(getLogOutput()).toContain("value=99999999999999999999");
    });
  });

  describe("metadata — object values", () => {
    it("serializes plain objects as quoted JSON", () => {
      makeLogger("Test").info("msg", { meta: { x: 1 } });
      expect(getLogOutput()).toContain('meta="{\\"x\\":1}"');
    });

    it("coerces nested bigints to strings inside object metadata", () => {
      // JSON.stringify would otherwise throw `TypeError: Do not know how to
      // serialize a BigInt` when encountering nested bigints.
      expect(() => makeLogger("Test").info("msg", { meta: { count: 100n } })).not.toThrow();
      expect(getLogOutput()).toContain('count\\":\\"100\\"');
    });

    it("does not throw on self-referential metadata", () => {
      const obj: Record<string, unknown> = { name: "node" };
      obj.self = obj;

      expect(() => makeLogger("Test").info("msg", { meta: obj })).not.toThrow();
      const output = getLogOutput();
      // First encounter still rendered; cyclic back-edge replaced.
      expect(output).toContain("node");
      expect(output).toContain("[Circular]");
    });

    it("does not throw on mutually-referential metadata", () => {
      const a: Record<string, unknown> = { id: "a" };
      const b: Record<string, unknown> = { id: "b" };
      a.peer = b;
      b.peer = a;

      expect(() => makeLogger("Test").info("msg", { meta: a })).not.toThrow();
      const output = getLogOutput();
      // Both nodes appear (the back-edge from b to a is what gets replaced).
      expect(output).toContain('id\\":\\"a');
      expect(output).toContain('id\\":\\"b');
      expect(output).toContain("[Circular]");
    });
  });

  describe("metadata — Error values", () => {
    it("outputs the error stack as an escaped single-line value", () => {
      const logger = makeLogger("Test");
      const error = new Error("something went wrong\nwith a newline");

      logger.error("oops", { error });

      const output = getLogOutput();
      expect(output).toContain('"Error: something went wrong\\nwith a newline');
      expect(
        output
          .trim()
          .split("\n")
          .filter((l) => l.includes("level=")),
      ).toHaveLength(1);
    });

    it("falls back to name and message when stack is unavailable", () => {
      const error = new Error("no stack");
      delete error.stack;

      makeLogger("Test").error("oops", { error });

      // Stackless Errors render as `${name}: ${message}` (no stack frames to append).
      expect(getLogOutput()).toContain("error=\"Error: no stack\"");
    });

    it("does not produce an error.stack key", () => {
      makeLogger("Test").error("oops", { error: new Error("msg") });
      expect(getLogOutput()).not.toContain("error.stack=");
    });

    it("unwraps Error.cause chains in metadata", () => {
      const root = new Error("root cause");
      const middle = new Error("middle", { cause: root });
      const top = new Error("top", { cause: middle });

      makeLogger("Test").error("oops", { error: top });

      const output = getLogOutput();
      expect(output).toContain("Error: top");
      // util.inspect renders nested causes via the `[cause]:` marker.
      expect(output).toContain("[cause]:");
      expect(output).toContain("Error: middle");
      expect(output).toContain("Error: root cause");
      // Single-line (newlines escaped to \\n by formatValue).
      expect(
        output
          .trim()
          .split("\n")
          .filter((l) => l.includes("level=")),
      ).toHaveLength(1);
    });

    it("renders non-Error causes inside the inspected output", () => {
      const err = new Error("boom");
      // Direct assignment makes `cause` enumerable, so util.inspect prints it
      // as a regular property (`cause: 'value'`, no square brackets).
      (err as Error & { cause?: unknown }).cause = "just a string";

      makeLogger("Test").error("oops", { error: err });

      const output = getLogOutput();
      expect(output).toContain("Error: boom");
      expect(output).toContain("cause: 'just a string'");
    });

    it("breaks self-referential cause cycles", () => {
      const a = new Error("A");
      const b = new Error("B");
      (a as Error & { cause?: unknown }).cause = b;
      (b as Error & { cause?: unknown }).cause = a;

      // Must terminate; util.inspect uses `<ref *N>` markers for cycles.
      makeLogger("Test").error("oops", { error: a });

      const output = getLogOutput();
      expect(output).toContain("Error: A");
      expect(output).toContain("Error: B");
    });

    it("unwraps AggregateError siblings", () => {
      const inner1 = new Error("first");
      const inner2 = new TypeError("second");
      const aggregate = new AggregateError([inner1, inner2], "all failed");

      makeLogger("Test").error("oops", { error: aggregate });

      const output = getLogOutput();
      expect(output).toContain("AggregateError: all failed");
      // Sibling errors are rendered under the `[errors]:` marker.
      expect(output).toContain("[errors]:");
      expect(output).toContain("Error: first");
      expect(output).toContain("TypeError: second");
    });

    it("redacts secret-named own-properties attached to an Error in metadata", () => {
      // Regression: RPC/HTTP libraries sometimes attach credentials to thrown
      // Error objects as own-properties. The Error renderer must route those
      // values through the per-key redaction pipeline, not dump them verbatim.
      const err = Object.assign(new Error("rpc failed"), {
        privateKey: "0xLEAK_PK",
        authorization: "Bearer LEAK_TOKEN",
        clientSecret: "LEAK_CLIENT_SECRET",
        code: "ECONNRESET",
      });

      makeLogger("Test").error("oops", { error: err });

      const output = getLogOutput();
      expect(output).not.toContain("0xLEAK_PK");
      expect(output).not.toContain("LEAK_TOKEN");
      expect(output).not.toContain("LEAK_CLIENT_SECRET");
      // Non-secret own-properties are still surfaced for operability.
      expect(output).toContain("ECONNRESET");
      expect(output).toContain("[REDACTED]");
    });

    it("redacts secrets nested inside an Error own-property object", () => {
      const err = Object.assign(new Error("config invalid"), {
        config: { password: "tsp-secret", host: "db.example" },
      });

      makeLogger("Test").error("oops", { error: err });

      const output = getLogOutput();
      expect(output).not.toContain("tsp-secret");
      expect(output).toContain("[REDACTED]");
      expect(output).toContain("db.example");
    });

    it("redacts secret-named own-properties on a nested cause Error", () => {
      const root = Object.assign(new Error("network down"), {
        accessToken: "leaky-access-token",
      });
      const top = new Error("request failed", { cause: root });

      makeLogger("Test").error("oops", { error: top });

      const output = getLogOutput();
      expect(output).not.toContain("leaky-access-token");
      expect(output).toContain("Error: network down");
    });

    it("redacts secret-named own-properties on AggregateError siblings", () => {
      const inner = Object.assign(new Error("inner"), {
        bearerToken: "leaky-bearer",
      });
      const aggregate = new AggregateError([inner], "all failed");

      makeLogger("Test").error("oops", { error: aggregate });

      const output = getLogOutput();
      expect(output).not.toContain("leaky-bearer");
      expect(output).toContain("Error: inner");
    });

    it("respects caller-supplied custom redact keys for Error own-properties", () => {
      const logger = makeLogger("Test", { redact: ["customSecret"] });
      const err = Object.assign(new Error("boom"), {
        customSecret: "should-not-leak-custom",
      });

      logger.error("oops", { error: err });

      const output = getLogOutput();
      expect(output).not.toContain("should-not-leak-custom");
      expect(output).toContain("[REDACTED]");
    });
  });

  describe("bare Error as message argument", () => {
    // Locks in the printf `if (stack)` branch (fed by winston's `errors({stack:true})`).
    it("renders stack under the error key when called as logger.error(err)", () => {
      const err = new Error("boom");
      makeLogger("Test").error(err);

      const output = getLogOutput();
      expect(output).toContain("msg=boom");
      expect(output).toContain("Error: boom");
      // Stack token appears in the error= field, not as a separate metadata key.
      expect(output).not.toContain("stack=");
    });

    it("surfaces Error.cause as a separate cause= metadata field", () => {
      // Winston's `errors({cause:true})` promotes `info.cause` so it flows
      // through formatMetadata. An Error cause is rendered via util.inspect.
      const root = new Error("network down");
      const top = new Error("request failed", { cause: root });

      makeLogger("Test").error(top);

      const output = getLogOutput();
      expect(output).toContain("Error: request failed");
      expect(output).toMatch(/cause="Error: network down/);
    });

    it("renders non-Error causes via formatMetadata", () => {
      const err = new Error("rpc failure");
      (err as Error & { cause?: unknown }).cause = "transport timeout";

      makeLogger("Test").error(err);

      const output = getLogOutput();
      expect(output).toContain("Error: rpc failure");
      // Non-Error cause is treated like any other string metadata value.
      expect(output).toContain('cause="transport timeout"');
    });
  });

  describe("child — static context merging", () => {
    it("returns a new logger that inherits the parent name", () => {
      const parent = makeLogger("Parent");
      const child = parent.child({ direction: "L1_TO_L2" });
      expect(child.name).toBe("Parent");
      expect(child).not.toBe(parent);
    });

    it("merges the static context into every log entry", () => {
      const parent = makeLogger("Parent");
      const child = parent.child({ direction: "L1_TO_L2" });
      child.info("hello");
      expect(getLogOutput()).toContain("direction=L1_TO_L2");
    });

    it("call-site metadata wins over child context on key collisions", () => {
      const parent = makeLogger("Parent");
      const child = parent.child({ direction: "L1_TO_L2" });
      child.info("hello", { direction: "L2_TO_L1" });
      const output = getLogOutput();
      expect(output).toContain("direction=L2_TO_L1");
      expect(output).not.toContain("direction=L1_TO_L2");
    });

    it("does not leak the static context into the parent logger", () => {
      const parent = makeLogger("Parent");
      parent.child({ direction: "L1_TO_L2" });
      parent.info("hello");
      expect(getLogOutput()).not.toContain("direction=");
    });

    it("composes nested children", () => {
      const parent = makeLogger("Parent");
      const grand = parent.child({ direction: "L1_TO_L2" }).child({ signerAddress: "0xabc" });
      grand.info("hello");
      const output = getLogOutput();
      expect(output).toContain("direction=L1_TO_L2");
      expect(output).toContain("signerAddress=0xabc");
    });
  });

  describe("redaction — sensitive keys", () => {
    it("redacts default sensitive keys at the top level (case-insensitive)", () => {
      const logger = makeLogger("Test");
      logger.info("login", {
        username: "alice",
        password: "hunter2",
        Authorization: "Bearer abcdef",
        ApiKey: "k-1234",
      });
      const output = getLogOutput();
      expect(output).toContain("username=alice");
      expect(output).toContain("password=[REDACTED]");
      expect(output).toContain("Authorization=[REDACTED]");
      expect(output).toContain("ApiKey=[REDACTED]");
      expect(output).not.toContain("hunter2");
      expect(output).not.toContain("Bearer abcdef");
      expect(output).not.toContain("k-1234");
    });

    it("redacts sensitive keys nested inside object metadata", () => {
      const logger = makeLogger("Test");
      logger.info("config loaded", {
        cfg: {
          host: "rpc.example.com",
          credentials: { privateKey: "0xdeadbeef", accessToken: "t-1" },
        },
      });
      const output = getLogOutput();
      expect(output).toContain("host"); // structural fields preserved
      expect(output).toContain("[REDACTED]");
      expect(output).not.toContain("0xdeadbeef");
      expect(output).not.toContain("t-1");
    });

    it("redacts OAuth-style token keys but not the bare domain term `token`", () => {
      // `token` in this monorepo is a domain term (ERC-20 token addresses,
      // TokenBridge, BridgedToken). Real bearer secrets live under more
      // specific keys (`accessToken`, `bearerToken`, `refreshToken`, `idToken`)
      // which must still be redacted.
      const logger = makeLogger("Test");
      logger.info("bridge transfer", {
        token: "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48", // ERC-20 address — non-sensitive
        accessToken: "secret-access",
        bearerToken: "secret-bearer",
        refresh_token: "secret-refresh",
        idToken: "secret-id",
      });
      const output = getLogOutput();
      expect(output).toContain("token=0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48");
      expect(output).toContain("accessToken=[REDACTED]");
      expect(output).toContain("bearerToken=[REDACTED]");
      expect(output).toContain("refresh_token=[REDACTED]");
      expect(output).toContain("idToken=[REDACTED]");
      expect(output).not.toContain("secret-access");
      expect(output).not.toContain("secret-bearer");
      expect(output).not.toContain("secret-refresh");
      expect(output).not.toContain("secret-id");
    });

    it("does not redact bare `auth` (domain term) but still redacts `authorization`", () => {
      // `auth` commonly names a non-sensitive auth mode/strategy/provider.
      // The actual HTTP header value is `authorization`.
      const logger = makeLogger("Test");
      logger.info("request", { auth: "oauth2", authorization: "Bearer secret-header" });
      const output = getLogOutput();
      expect(output).toContain("auth=oauth2");
      expect(output).toContain("authorization=[REDACTED]");
      expect(output).not.toContain("secret-header");
    });

    it("redacts via additional caller-supplied keys merged with defaults", () => {
      const logger = makeLogger("Test", { redact: ["customSecret"] });
      logger.info("msg", { customSecret: "do-not-leak", password: "p", safe: "ok" });
      const output = getLogOutput();
      expect(output).toContain("customSecret=[REDACTED]");
      expect(output).toContain("password=[REDACTED]");
      expect(output).toContain("safe=ok");
      expect(output).not.toContain("do-not-leak");
    });

    it("propagates redaction config to child loggers", () => {
      const parent = makeLogger("Parent", { redact: ["customSecret"] });
      const child = parent.child({ direction: "L1_TO_L2" });
      child.info("hello", { customSecret: "leaked?", password: "x" });
      const output = getLogOutput();
      expect(output).toContain("customSecret=[REDACTED]");
      expect(output).toContain("password=[REDACTED]");
      expect(output).not.toContain("leaked?");
    });

    it("does not redact keys that merely contain a sensitive substring", () => {
      const logger = makeLogger("Test");
      // "passport" should NOT be redacted just because "pass" is a prefix.
      logger.info("msg", { passport: "p-123", payload: "ok" });
      const output = getLogOutput();
      expect(output).toContain("passport=p-123");
      expect(output).toContain("payload=ok");
    });

    it("redacts OAuth-style client secret variants", () => {
      const logger = makeLogger("Test");
      logger.info("oauth", {
        clientSecret: "cs-1",
        client_secret: "cs-2",
        secretKey: "sk-1",
        secret_key: "sk-2",
      });
      const output = getLogOutput();
      expect(output).toContain("clientSecret=[REDACTED]");
      expect(output).toContain("client_secret=[REDACTED]");
      expect(output).toContain("secretKey=[REDACTED]");
      expect(output).toContain("secret_key=[REDACTED]");
      expect(output).not.toContain("cs-1");
      expect(output).not.toContain("cs-2");
      expect(output).not.toContain("sk-1");
      expect(output).not.toContain("sk-2");
    });

    it("redacts TLS keystore/truststore password variants", () => {
      const logger = makeLogger("Test");
      logger.info("tls", {
        keyStorePassword: "ksp-1",
        keystore_password: "ksp-2",
        trustStorePassword: "tsp-1",
        truststore_password: "tsp-2",
      });
      const output = getLogOutput();
      expect(output).toContain("keyStorePassword=[REDACTED]");
      expect(output).toContain("keystore_password=[REDACTED]");
      expect(output).toContain("trustStorePassword=[REDACTED]");
      expect(output).toContain("truststore_password=[REDACTED]");
      expect(output).not.toContain("ksp-1");
      expect(output).not.toContain("ksp-2");
      expect(output).not.toContain("tsp-1");
      expect(output).not.toContain("tsp-2");
    });

    it("redacts secrets when passed as a bare Error to logger.error()", () => {
      // The bare-Error path runs the Error through winston's
      // `errors({ stack: true })` which `Object.assign`s its enumerable own
      // properties onto the `info` object. Each property then flows through
      // `formatMetadata`, which masks redacted keys per-key.
      const err = Object.assign(new Error("startup failed"), {
        clientSecret: "should-not-leak",
      });
      makeLogger("Test").error(err);
      const output = getLogOutput();
      expect(output).toContain("clientSecret=[REDACTED]");
      expect(output).not.toContain("should-not-leak");
    });

    it("ignores caller-supplied `format` so redaction cannot be bypassed", () => {
      // Regression: a caller-supplied `format` must NOT replace the internal
      // pipeline. If it did, every secret would print verbatim because the
      // user's format wouldn't run our redaction printf. The constructor
      // applies caller options first, then overrides `format` with our own.
      const logger = makeLogger("Test", { format: format.json() });
      logger.info("attempted bypass", { password: "should-not-leak" });
      const output = getLogOutput();
      expect(output).toContain("password=[REDACTED]");
      expect(output).not.toContain("should-not-leak");
      // Our logfmt pipeline ran (not the user's JSON format).
      expect(output).toMatch(/level=INFO/);
      expect(output).toMatch(/logger=Test/);
    });

    it("respects caller-supplied `level` (still overridable)", () => {
      // Sanity check that the spread reorder didn't break the existing
      // contract: callers set `level` to control verbosity.
      const logger = makeLogger("Test", { level: "debug" });
      logger.debug("visible at debug");
      expect(getLogOutput()).toContain("visible at debug");
    });
  });

  describe("URL stripping in Error text", () => {
    // viem/ethers/HTTP libs bake the request URL into `error.message`, which
    // also appears in the first line of `error.stack`. Per-key redaction
    // can't reach those — the printf and metadata Error renderer strip URL
    // substrings from the rendered text before it leaves the logger.
    it("redacts URL inside RpcRequestError-style message when logged as bare Error", () => {
      const err = new Error("RPC Request failed.\n\n    URL: https://rpc.devnet.linea.build");
      err.name = "RpcRequestError";
      makeLogger("Test").error(err);
      const output = getLogOutput();
      expect(output).toContain("[REDACTED_URL]");
      expect(output).not.toContain("rpc.devnet.linea.build");
    });

    it("redacts URL inside Error message when logged as metadata", () => {
      const err = new Error("RPC Request failed.\n\n    URL: https://rpc.devnet.linea.build");
      makeLogger("Test").error("upstream", { error: err });
      const output = getLogOutput();
      expect(output).toContain("[REDACTED_URL]");
      expect(output).not.toContain("rpc.devnet.linea.build");
    });

    it("redacts URL with userinfo and query params inside Error message", () => {
      const err = new Error(
        "request failed: GET https://user:secret@mainnet.infura.io/v3/abc?apiKey=k-123 returned 401",
      );
      makeLogger("Test").error("rpc", { error: err });
      const output = getLogOutput();
      expect(output).toContain("[REDACTED_URL]");
      expect(output).not.toContain("user:secret");
      expect(output).not.toContain("mainnet.infura.io");
      expect(output).not.toContain("k-123");
    });

    it("redacts URLs inside the Error stack frames (not just the first line)", () => {
      // Assemble a stack-like string by concatenation rather than relying on
      // V8's stack capture, so the test doesn't depend on file paths.
      const err = new Error("boom");
      err.stack = [
        "Error: boom",
        "    at handler (https://internal.example.org/lib/handler.js:42:10)",
        "    at Object.<anonymous> (/repo/src/main.ts:1:1)",
      ].join("\n");
      makeLogger("Test").error(err);
      const output = getLogOutput();
      expect(output).toContain("[REDACTED_URL]");
      expect(output).not.toContain("internal.example.org");
      // Local file paths in the stack are not URL-shaped and must survive.
      expect(output).toContain("/repo/src/main.ts");
    });

    it("leaves non-URL text untouched", () => {
      const err = new Error("simple validation error: field 'foo' missing");
      makeLogger("Test").error("oops", { error: err });
      const output = getLogOutput();
      expect(output).not.toContain("[REDACTED_URL]");
      expect(output).toContain("simple validation error");
    });
  });

  describe("printf-style interpolation (splat)", () => {
    // The `ILogger` signature `info(message: any, ...params: any[])` advertises
    // Node's `util.format`-style interpolation. Removing `splat()` from the
    // format pipeline is a silent, backward-incompatible change for any consumer
    // of this shared library that writes `logger.info("value is %s", val)`.
    // These tests lock the pipeline so the token is interpolated instead of
    // leaking into the output verbatim.
    it("interpolates %s with a string argument", () => {
      makeLogger("Test").info("value is %s", "hello");
      const output = getLogOutput();
      expect(output).toContain('msg="value is hello"');
      expect(output).not.toContain("%s");
    });

    it("interpolates %d with a number argument", () => {
      makeLogger("Test").info("count is %d", 42);
      const output = getLogOutput();
      expect(output).toContain('msg="count is 42"');
      expect(output).not.toContain("%d");
    });

    it("interpolates multiple positional tokens in order", () => {
      makeLogger("Test").info("%s took %d ms", "op", 123);
      expect(getLogOutput()).toContain('msg="op took 123 ms"');
    });

    it("still merges trailing object metadata after interpolation", () => {
      // `logger.info("msg %s", "x", { foo: "bar" })`: splat consumes the first
      // positional arg for `%s`, Winston's core merges the trailing object as
      // metadata.
      makeLogger("Test").info("op %s", "start", { foo: "bar" });
      const output = getLogOutput();
      expect(output).toContain('msg="op start"');
      expect(output).toContain("foo=bar");
    });
  });

  describe("normalizeParams — bare Error as second argument", () => {
    it("does not corrupt the msg field", () => {
      const logger = makeLogger("Test");
      logger.error("something failed", new Error("direct error"));

      const output = getLogOutput();
      expect(output).toContain('msg="something failed"');
      expect(output).not.toContain("something failed direct error");
    });

    it("outputs the stack under the error key", () => {
      makeLogger("Test").error("something failed", new Error("direct error"));
      expect(getLogOutput()).toContain("Error: direct error");
    });

    it("does not scatter enumerable Error properties into metadata", () => {
      const error = Object.assign(new Error("conn refused"), { code: "ECONNREFUSED", errno: -61 });
      makeLogger("Test").error("startup failed", error);

      const output = getLogOutput();
      expect(output).not.toContain("code=ECONNREFUSED");
      expect(output).not.toContain("errno=-61");
    });
  });
});
