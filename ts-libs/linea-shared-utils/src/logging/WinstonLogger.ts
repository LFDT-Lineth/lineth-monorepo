/* eslint-disable @typescript-eslint/no-explicit-any */
import { inspect } from "node:util";
import { Logger as LoggerClass, LoggerOptions, createLogger, format, transports } from "winston";

import { ILogger } from "./ILogger";

// Logfmt: quote values that contain whitespace, double-quote, equals, or backslash.
const NEEDS_QUOTE_RE = /[ \t\n\r"=\\]/;
// Characters that must be escaped inside a quoted logfmt value.
const ESCAPE_RE = /["\\\n\r\t]/g;
const ESCAPE_MAP: Record<string, string> = {
  '"': '\\"',
  "\\": "\\\\",
  "\n": "\\n",
  "\r": "\\r",
  "\t": "\\t",
};

const REDACTED_PLACEHOLDER = "[REDACTED]";
const URL_REPLACEMENT = "[REDACTED_URL]";

// Bounds traversal of pathological Error structures (deep `cause` chains,
// large AggregateError sibling lists). Cycles are broken via `seen`.
const ERROR_RENDER_MAX_DEPTH = 5;
const ERROR_RENDER_MAX_AGGREGATE = 100;

// Own-property names that the Error renderer handles structurally and must
// NOT re-emit as part of the generic own-properties dump.
const ERROR_STRUCTURAL_KEYS: ReadonlySet<string> = new Set(["name", "message", "stack", "cause", "errors"]);

/**
 * Default keys whose values are masked in log output. Comparison is
 * case-insensitive and exact (no substring matching, to avoid e.g. "passport"
 * matching "pass"). Keys are stored lower-cased.
 */
const DEFAULT_REDACT_KEYS: ReadonlyArray<string> = [
  "privatekey",
  "private_key",
  "signerkey",
  "signer_key",
  "secretkey",
  "secret_key",
  "clientsecret",
  "client_secret",
  "password",
  "passphrase",
  "keystorepassword",
  "keystore_password",
  "truststorepassword",
  "truststore_password",
  "apikey",
  "api_key",
  "mnemonic",
  "secret",
  "accesstoken",
  "access_token",
  "bearertoken",
  "bearer_token",
  "refreshtoken",
  "refresh_token",
  "idtoken",
  "id_token",
  "authorization",
  "seed",
];

/**
 * Matches URL substrings inside free-form text. Used to scrub URLs from
 * Error messages and stacks (e.g. viem's
 * `RpcRequestError: RPC Request failed.\n\n  URL: https://rpc.example/path`),
 * where per-key redaction can't reach.
 *
 * Limited to schemes actually used in this monorepo so it does not match
 * `file:` paths or arbitrary `foo://bar` identifiers. Match stops at
 * whitespace/quotes/angle-brackets so the surrounding text is preserved
 * verbatim.
 */
const URL_PATTERN_RE = /\b(?:https?|wss?|postgres(?:ql)?|mysql|mongodb|redis):\/\/[^\s"'<>`\\]+/gi;

export type WinstonLoggerOptions = LoggerOptions & {
  /**
   * Additional keys (case-insensitive) whose values must be replaced with
   * `[REDACTED]` in log metadata. Merged with the built-in default list.
   */
  redact?: string[];
};

function formatValue(value: string): string {
  if (!NEEDS_QUOTE_RE.test(value)) return value;
  return `"${value.replace(ESCAPE_RE, (c) => ESCAPE_MAP[c])}"`;
}

/**
 * Replaces every URL substring in `text` with `[REDACTED_URL]`. Used on
 * Error `message` and `stack` strings, so URLs embedded by HTTP/RPC libraries
 * (viem, ethers, etc.) — which often carry userinfo or `?apiKey=…` — never
 * reach the log output. `String#replace` with a global regex returns the
 * original string when there are no matches, so no fast-path needed.
 */
function stripUrlsFromText(text: string): string {
  return text.replace(URL_PATTERN_RE, URL_REPLACEMENT);
}

export class WinstonLogger implements ILogger {
  private logger: LoggerClass;
  public readonly name: string;
  private readonly redactKeys: ReadonlySet<string>;

  constructor(loggerName: string, options?: WinstonLoggerOptions) {
    const { combine, colorize, timestamp, printf, errors, label, splat } = format;
    const colorizer = colorize();

    const { redact, ...winstonOptions } = options ?? {};
    const keys = new Set<string>(DEFAULT_REDACT_KEYS);
    if (redact) for (const k of redact) keys.add(k.toLowerCase());
    this.redactKeys = keys;

    // Spread caller options FIRST, then override with our defaults / required
    // fields. `format` is non-overridable: it owns the redaction and URL-strip
    // pipeline, so a caller-supplied format must never replace it (otherwise
    // every secret leaks). `transports` IS overridable so tests can swap in a
    // capturing stream — transports receive already-formatted strings and
    // therefore cannot bypass redaction. `level` falls back to "info" only if
    // the caller didn't set one.
    this.logger = createLogger({
      ...winstonOptions,
      level: winstonOptions.level ?? "info",
      transports: winstonOptions.transports ?? [new transports.Console()],
      format: combine(
        timestamp(),
        // For bare-Error calls (`logger.error(err)`):
        //  - `stack: true` promotes `err.stack` so printf can emit `error=…`.
        //  - `cause: true` promotes `err.cause` so it surfaces as a separate
        //    `cause=…` metadata field. `cause` is non-enumerable when set
        //    via `new Error(msg, { cause })`, so without this flag winston's
        //    `Object.assign` would silently drop it.
        errors({ stack: true, cause: true }),
        splat(),
        label({ label: loggerName }),
        printf(({ timestamp, level, label, message, stack, ...metadata }) => {
          const coloredLevel = colorizer.colorize(level, level.toUpperCase());
          // `message` is free-form text. Strip URLs so `RpcRequestError`-style
          // errors (which embed the request URL into `error.message`, and
          // therefore into the first line of `error.stack`) cannot leak them.
          const safeMessage = stripUrlsFromText(String(message));
          let str = `time=${timestamp} level=${coloredLevel} logger=${label} msg=${formatValue(safeMessage)}`;

          const meta = this.formatMetadata(metadata);
          if (meta) str += ` ${meta}`;

          if (stack) {
            const safeStack = stripUrlsFromText(String(stack));
            str += ` error=${formatValue(safeStack)}`;
          }

          return str;
        }),
      ),
    });
    this.name = loggerName;
  }

  private isRedacted(key: string): boolean {
    return this.redactKeys.has(key.toLowerCase());
  }

  /**
   * Walks a plain object/array and returns a structurally equivalent copy
   * with values under any redacted key replaced with `[REDACTED]`. Bigints
   * are coerced to strings (so `JSON.stringify` doesn't throw); cycles are
   * broken with `"[Circular]"`.
   *
   * Scope is intentionally narrow: per-key redaction for caller-supplied
   * config dumps. URLs embedded inside string values are NOT scrubbed here —
   * those are caught at the Error-message/stack layer in the printf.
   */
  private redactValue(value: unknown, key: string, seen: WeakSet<object>): unknown {
    if (key && this.isRedacted(key)) return REDACTED_PLACEHOLDER;
    if (typeof value === "bigint") return value.toString();
    if (value === null || typeof value !== "object") return value;
    if (seen.has(value)) return "[Circular]";
    seen.add(value);

    if (Array.isArray(value)) {
      return value.map((item, i) => this.redactValue(item, String(i), seen));
    }
    const out: Record<string, unknown> = {};
    for (const propKey of Object.keys(value)) {
      out[propKey] = this.redactValue((value as Record<string, unknown>)[propKey], propKey, seen);
    }
    return out;
  }

  private formatMetadataValue(value: unknown): string {
    if (value == null) return "null";
    if (typeof value === "string") return formatValue(value);
    if (typeof value === "number" || typeof value === "boolean") return String(value);
    if (typeof value === "bigint") return value.toString();
    if (value instanceof Error) {
      // Render the Error structurally so own-properties (e.g. fields attached
      // by RPC/HTTP libs like `privateKey`, `authorization`, `clientSecret`)
      // pass through `redactValue` instead of leaking via `util.inspect`'s
      // verbatim property dump. URL-strip is applied to the final text so
      // URLs embedded in `message`/`stack` (viem/ethers RpcRequestError) are
      // also scrubbed.
      return formatValue(stripUrlsFromText(this.renderError(value, new WeakSet<object>(), 0)));
    }
    return formatValue(JSON.stringify(this.redactValue(value, "", new WeakSet<object>())));
  }

  /**
   * Renders an `Error` into a single string that preserves the shape callers
   * expect (`Name: message\n    at ...`, `[cause]:` markers for nested causes,
   * `[errors]: [ ... ]` for AggregateError siblings) while ensuring every
   * own-property value flows through the per-key redaction pipeline.
   *
   * Emits a leading `${name}: ${message}` only when `err.stack` does not
   * already start with it — Node's default `Error.stack` begins with that
   * line, but custom errors (e.g. from `Error.captureStackTrace` overrides)
   * may not. `cause` and `errors` are pulled directly from the object
   * regardless of enumerability so `new Error(msg, { cause })` still surfaces.
   */
  private renderError(err: Error, seen: WeakSet<object>, depth: number): string {
    if (seen.has(err)) return "[Circular]";
    seen.add(err);

    const header = this.renderErrorHeader(err);
    const lines: string[] = [header];

    const ownProps = this.renderErrorOwnProps(err);
    if (ownProps) lines.push(`  ${ownProps}`);

    if (depth < ERROR_RENDER_MAX_DEPTH) {
      const causeLine = this.renderCause(err, seen, depth);
      if (causeLine) lines.push(`  ${causeLine}`);

      const errorsLine = this.renderAggregateErrors(err, seen, depth);
      if (errorsLine) lines.push(`  ${errorsLine}`);
    }

    return lines.join("\n");
  }

  private renderErrorHeader(err: Error): string {
    const name = err.name || "Error";
    const message = String(err.message ?? "");
    const headerLine = `${name}: ${message}`;
    if (typeof err.stack === "string" && err.stack.length > 0) {
      // Node's default stack already starts with `${name}: ${message}\n    at ...`.
      // If a runtime supplies just the frames (no header), prepend ours.
      return err.stack.startsWith(headerLine) ? err.stack : `${headerLine}\n${err.stack}`;
    }
    return headerLine;
  }

  private renderErrorOwnProps(err: Error): string {
    // `cause`, `errors`, and the standard `name`/`message`/`stack` slots are
    // handled structurally; everything else (e.g. `code`, `errno`, plus any
    // secrets a library attached) is redacted per-key.
    const ownPropEntries: Record<string, unknown> = {};
    let hasOwnProps = false;
    for (const key of Object.keys(err)) {
      if (ERROR_STRUCTURAL_KEYS.has(key)) continue;
      ownPropEntries[key] = (err as unknown as Record<string, unknown>)[key];
      hasOwnProps = true;
    }
    if (!hasOwnProps) return "";
    const redacted = this.redactValue(ownPropEntries, "", new WeakSet<object>()) as Record<string, unknown>;
    return JSON.stringify(redacted);
  }

  private renderCause(err: Error, seen: WeakSet<object>, depth: number): string {
    if (!("cause" in err)) return "";
    const cause = (err as Error & { cause?: unknown }).cause;
    if (cause === undefined) return "";
    if (cause instanceof Error) {
      return `[cause]: ${this.renderError(cause, seen, depth + 1)}`;
    }
    // Non-Error cause: redact then `inspect` the leaf so single-quoted strings
    // ('value') match historical output. `customInspect: false` prevents a
    // hostile object from injecting its own representation.
    const redacted = this.redactValue(cause, "cause", new WeakSet<object>());
    return `cause: ${inspect(redacted, { depth: 5, colors: false, breakLength: Infinity, customInspect: false })}`;
  }

  private renderAggregateErrors(err: Error, seen: WeakSet<object>, depth: number): string {
    const siblings = (err as Error & { errors?: unknown }).errors;
    if (!Array.isArray(siblings) || siblings.length === 0) return "";
    const limit = Math.min(siblings.length, ERROR_RENDER_MAX_AGGREGATE);
    const rendered: string[] = [];
    for (let i = 0; i < limit; i++) {
      const item = siblings[i];
      if (item instanceof Error) {
        rendered.push(this.renderError(item, seen, depth + 1));
      } else {
        const redacted = this.redactValue(item, "", new WeakSet<object>());
        rendered.push(inspect(redacted, { depth: 5, colors: false, breakLength: Infinity, customInspect: false }));
      }
    }
    if (siblings.length > limit) rendered.push(`... ${siblings.length - limit} more`);
    return `[errors]: [ ${rendered.join(", ")} ]`;
  }

  private formatMetadata(metadata: Record<string, unknown>): string {
    const keys = Object.keys(metadata);
    if (keys.length === 0) return "";
    const parts = new Array<string>(keys.length);
    for (let i = 0; i < keys.length; i++) {
      const key = keys[i];
      parts[i] = `${key}=${this.isRedacted(key) ? REDACTED_PLACEHOLDER : this.formatMetadataValue(metadata[key])}`;
    }
    return parts.join(" ");
  }

  private normalizeParams(params: any[]): any[] {
    if (params.length === 1 && params[0] instanceof Error) {
      return [{ error: params[0] }];
    }
    return params;
  }

  public info(message: any, ...params: any[]): void {
    this.logger.info(message, ...this.normalizeParams(params));
  }

  public error(message: any, ...params: any[]): void {
    this.logger.error(message, ...this.normalizeParams(params));
  }

  public warn(message: any, ...params: any[]): void {
    this.logger.warn(message, ...this.normalizeParams(params));
  }

  public debug(message: any, ...params: any[]): void {
    this.logger.debug(message, ...this.normalizeParams(params));
  }

  public child(context: Record<string, unknown>): WinstonLogger {
    // Skip the constructor (which would build a fresh winston pipeline) and
    // wrap the parent's already-configured `winston.Logger.child()` directly.
    // Plain assignment matches the constructor's class-field semantics
    // (enumerable + writable); TypeScript's `readonly` is compile-time only.
    return Object.assign(Object.create(WinstonLogger.prototype) as WinstonLogger, {
      name: this.name,
      logger: this.logger.child(context),
      redactKeys: this.redactKeys,
    });
  }
}
