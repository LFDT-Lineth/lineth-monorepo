/**
 * Static extract of Postman env-var config from envLoader.ts correlated with schema.ts.
 * No TypeScript compile / runtime of Postman.
 */

const fs = require("node:fs");
const path = require("node:path");

const ENV_LOADER_REL = "postman/src/application/postman/app/config/envLoader.ts";
const SCHEMA_REL = "postman/src/application/postman/app/config/schema.ts";

const SECTION_META = [
  { id: "general", title: "General", match: (k) => false },
  { id: "l1-network", title: "L1 network", match: (k) => false },
  { id: "l2-network", title: "L2 network", match: (k) => false },
  { id: "listener", title: "Listener", match: (k) => false },
  { id: "claiming", title: "Claiming", match: (k) => false },
  { id: "signer", title: "Signer", match: (k) => false },
  { id: "database", title: "Database", match: (k) => false },
  { id: "database-cleaner", title: "Database cleaner", match: (k) => false },
  { id: "api", title: "API", match: (k) => false },
];

/** Placeholder defaults that must not be published as real defaults. */
const PLACEHOLDER_DEFAULTS = new Set(["", "0x"]);

/**
 * Env inventory driven by envLoader.ts structure.
 * expand: "prefix" → L1_ and L2_; "sponsoring" → opposite_prefix form;
 * shared dual-writes stay as a single row.
 *
 * defaultKind:
 *   literal:<value> — publish
 *   placeholder:<value> — blank + flag
 *   bool-false — === "true" pattern
 *   optional — optionalInt/BigInt/Float / omit
 *   conditional — optional with condition note
 */
const ENV_INVENTORY = [
  // general
  {
    section: "general",
    env: "LOG_LEVEL",
    field: "loggerOptions.level",
    schemaField: "loggerOptions",
    typeHint: "string",
    description: "Log level for the Winston logger",
    defaultKind: "literal:info",
    required: false,
  },
  {
    section: "general",
    env: "L1_L2_AUTO_CLAIM_ENABLED",
    field: "l1L2AutoClaimEnabled",
    schemaField: "l1L2AutoClaimEnabled",
    defaultKind: "bool-false",
    required: true,
  },
  {
    section: "general",
    env: "L2_L1_AUTO_CLAIM_ENABLED",
    field: "l2L1AutoClaimEnabled",
    schemaField: "l2L1AutoClaimEnabled",
    defaultKind: "bool-false",
    required: true,
  },

  // l1-network
  {
    section: "l1-network",
    env: "L1_RPC_URL",
    field: "l1Options.rpcUrl",
    schemaField: "rpcUrl",
    defaultKind: "placeholder:",
    required: true,
    secret: true,
  },
  {
    section: "l1-network",
    env: "L1_CONTRACT_ADDRESS",
    field: "l1Options.messageServiceContractAddress",
    schemaField: "messageServiceContractAddress",
    defaultKind: "placeholder:",
    required: true,
    secret: true,
  },
  {
    section: "l1-network",
    env: "L1_L2_EOA_ENABLED",
    field: "l1Options.isEOAEnabled",
    schemaField: "isEOAEnabled",
    defaultKind: "bool-false",
    required: false,
  },
  {
    section: "l1-network",
    env: "L1_L2_CALLDATA_ENABLED",
    field: "l1Options.isCalldataEnabled",
    schemaField: "isCalldataEnabled",
    defaultKind: "bool-false",
    required: false,
  },

  // l2-network
  {
    section: "l2-network",
    env: "L2_RPC_URL",
    field: "l2Options.rpcUrl",
    schemaField: "rpcUrl",
    defaultKind: "placeholder:",
    required: true,
    secret: true,
  },
  {
    section: "l2-network",
    env: "L2_CONTRACT_ADDRESS",
    field: "l2Options.messageServiceContractAddress",
    schemaField: "messageServiceContractAddress",
    defaultKind: "placeholder:",
    required: true,
    secret: true,
  },
  {
    section: "l2-network",
    env: "L2_L1_EOA_ENABLED",
    field: "l2Options.isEOAEnabled",
    schemaField: "isEOAEnabled",
    defaultKind: "bool-false",
    required: false,
  },
  {
    section: "l2-network",
    env: "L2_L1_CALLDATA_ENABLED",
    field: "l2Options.isCalldataEnabled",
    schemaField: "isCalldataEnabled",
    defaultKind: "bool-false",
    required: false,
  },
  {
    section: "l2-network",
    env: "L2_MESSAGE_TREE_DEPTH",
    field: "l2Options.l2MessageTreeDepth",
    schemaField: "l2MessageTreeDepth",
    defaultKind: "optional",
    required: false,
  },
  {
    section: "l2-network",
    env: "ENABLE_LINEA_ESTIMATE_GAS",
    field: "l2Options.enableLineaEstimateGas",
    schemaField: "enableLineaEstimateGas",
    defaultKind: "bool-false",
    required: false,
  },

  // listener — shared
  {
    section: "listener",
    env: "MAX_FETCH_MESSAGES_FROM_DB",
    field: "listener.maxFetchMessagesFromDb",
    schemaField: "maxFetchMessagesFromDb",
    defaultKind: "optional",
    required: false,
  },
  // listener — prefixed
  {
    section: "listener",
    expand: "prefix",
    envSuffix: "LISTENER_INTERVAL",
    field: "listener.pollingInterval",
    schemaField: "pollingInterval",
    defaultKind: "optional",
    required: false,
  },
  {
    section: "listener",
    expand: "prefix",
    envSuffix: "LISTENER_RECEIPT_POLLING_INTERVAL",
    field: "listener.receiptPollingInterval",
    schemaField: "receiptPollingInterval",
    defaultKind: "optional",
    required: false,
  },
  {
    section: "listener",
    expand: "prefix",
    envSuffix: "MAX_BLOCKS_TO_FETCH_LOGS",
    field: "listener.maxBlocksToFetchLogs",
    schemaField: "maxBlocksToFetchLogs",
    defaultKind: "optional",
    required: false,
  },
  {
    section: "listener",
    expand: "prefix",
    envSuffix: "LISTENER_INITIAL_FROM_BLOCK",
    field: "listener.initialFromBlock",
    schemaField: "initialFromBlock",
    defaultKind: "conditional",
    condition: "included only when parseInt(value) >= 0",
    required: false,
  },
  {
    section: "listener",
    expand: "prefix",
    envSuffix: "LISTENER_BLOCK_CONFIRMATION",
    field: "listener.blockConfirmation",
    schemaField: "blockConfirmation",
    defaultKind: "conditional",
    condition: "included only when parseInt(value) >= 0",
    required: false,
  },
  {
    section: "listener",
    expand: "prefix",
    envSuffix: "EVENT_FILTER_FROM_ADDRESS",
    field: "listener.eventFilters.fromAddressFilter",
    schemaField: "fromAddressFilter",
    defaultKind: "conditional",
    condition: "eventFilters object included only when any filter is set",
    required: false,
    secret: true,
  },
  {
    section: "listener",
    expand: "prefix",
    envSuffix: "EVENT_FILTER_TO_ADDRESS",
    field: "listener.eventFilters.toAddressFilter",
    schemaField: "toAddressFilter",
    defaultKind: "conditional",
    condition: "eventFilters object included only when any filter is set",
    required: false,
    secret: true,
  },
  {
    section: "listener",
    expand: "prefix",
    envSuffix: "EVENT_FILTER_CALLDATA",
    field: "listener.eventFilters.calldataFilter.criteriaExpression",
    schemaField: "criteriaExpression",
    defaultKind: "conditional",
    condition: "requires both CALLDATA and CALLDATA_FUNCTION_INTERFACE",
    required: false,
  },
  {
    section: "listener",
    expand: "prefix",
    envSuffix: "EVENT_FILTER_CALLDATA_FUNCTION_INTERFACE",
    field: "listener.eventFilters.calldataFilter.calldataFunctionInterface",
    schemaField: "calldataFunctionInterface",
    defaultKind: "conditional",
    condition: "requires both CALLDATA and CALLDATA_FUNCTION_INTERFACE",
    required: false,
  },

  // claiming — shared
  {
    section: "claiming",
    env: "MESSAGE_SUBMISSION_TIMEOUT",
    field: "claiming.messageSubmissionTimeout",
    schemaField: "messageSubmissionTimeout",
    defaultKind: "optional",
    required: false,
  },
  {
    section: "claiming",
    env: "MAX_NONCE_DIFF",
    field: "claiming.maxNonceDiff",
    schemaField: "maxNonceDiff",
    defaultKind: "optional",
    required: false,
  },
  {
    section: "claiming",
    env: "MAX_FEE_PER_GAS_CAP",
    field: "claiming.maxFeePerGasCap",
    schemaField: "maxFeePerGasCap",
    defaultKind: "optional",
    required: false,
  },
  {
    section: "claiming",
    env: "GAS_ESTIMATION_PERCENTILE",
    field: "claiming.gasEstimationPercentile",
    schemaField: "gasEstimationPercentile",
    defaultKind: "optional",
    required: false,
  },
  {
    section: "claiming",
    env: "PROFIT_MARGIN",
    field: "claiming.profitMargin",
    schemaField: "profitMargin",
    defaultKind: "optional",
    required: false,
  },
  {
    section: "claiming",
    env: "MAX_NUMBER_OF_RETRIES",
    field: "claiming.maxNumberOfRetries",
    schemaField: "maxNumberOfRetries",
    defaultKind: "optional",
    required: false,
  },
  {
    section: "claiming",
    env: "RETRY_DELAY_IN_SECONDS",
    field: "claiming.retryDelayInSeconds",
    schemaField: "retryDelayInSeconds",
    defaultKind: "optional",
    required: false,
  },
  {
    section: "claiming",
    env: "MAX_CLAIM_GAS_LIMIT",
    field: "claiming.maxClaimGasLimit",
    schemaField: "maxClaimGasLimit",
    defaultKind: "optional",
    required: false,
  },
  {
    section: "claiming",
    env: "MAX_BUMPS_PER_CYCLE",
    field: "claiming.maxBumpsPerCycle",
    schemaField: "maxBumpsPerCycle",
    defaultKind: "optional",
    required: false,
  },
  {
    section: "claiming",
    env: "MAX_RETRY_CYCLES",
    field: "claiming.maxRetryCycles",
    schemaField: "maxRetryCycles",
    defaultKind: "optional",
    required: false,
  },
  {
    section: "claiming",
    env: "MAX_POSTMAN_SPONSOR_GAS_LIMIT",
    field: "claiming.maxPostmanSponsorGasLimit",
    schemaField: "maxPostmanSponsorGasLimit",
    defaultKind: "optional",
    required: false,
  },
  {
    section: "claiming",
    expand: "prefix",
    envSuffix: "MAX_GAS_FEE_ENFORCED",
    field: "claiming.isMaxGasFeeEnforced",
    schemaField: "isMaxGasFeeEnforced",
    defaultKind: "bool-false",
    required: false,
  },
  {
    section: "claiming",
    expand: "prefix",
    envSuffix: "CLAIM_VIA_ADDRESS",
    field: "claiming.claimViaAddress",
    schemaField: "claimViaAddress",
    defaultKind: "optional",
    required: false,
    secret: true,
  },
  {
    section: "claiming",
    expand: "sponsoring",
    field: "claiming.isPostmanSponsorshipEnabled",
    schemaField: "isPostmanSponsorshipEnabled",
    defaultKind: "bool-false",
    required: false,
  },

  // signer
  {
    section: "signer",
    expand: "prefix",
    envSuffix: "SIGNER_TYPE",
    field: "claiming.signer.type",
    schemaField: "signerType",
    defaultKind: "literal:private-key",
    required: true,
    oneof: ["private-key", "web3signer", "aws-kms"],
  },
  {
    section: "signer",
    expand: "prefix",
    envSuffix: "SIGNER_PRIVATE_KEY",
    field: "claiming.signer.privateKey",
    schemaField: "privateKey",
    defaultKind: "placeholder:0x",
    required: false,
    secret: true,
    condition: 'used when SIGNER_TYPE is "private-key"',
  },
  {
    section: "signer",
    expand: "prefix",
    envSuffix: "WEB3_SIGNER_ENDPOINT",
    field: "claiming.signer.endpoint",
    schemaField: "endpoint",
    defaultKind: "placeholder:",
    required: false,
    secret: true,
    condition: 'used when SIGNER_TYPE is "web3signer"',
  },
  {
    section: "signer",
    expand: "prefix",
    envSuffix: "WEB3_SIGNER_PUBLIC_KEY",
    field: "claiming.signer.publicKey",
    schemaField: "publicKey",
    defaultKind: "placeholder:0x",
    required: false,
    secret: true,
    condition: 'used when SIGNER_TYPE is "web3signer"',
  },
  {
    section: "signer",
    expand: "prefix",
    envSuffix: "WEB3_SIGNER_TLS_KEYSTORE_PATH",
    field: "claiming.signer.tls.keyStorePath",
    schemaField: "keyStorePath",
    defaultKind: "conditional",
    condition: "TLS object included only when this path is set",
    required: false,
    secret: true,
  },
  {
    section: "signer",
    expand: "prefix",
    envSuffix: "WEB3_SIGNER_TLS_KEYSTORE_PASSWORD",
    field: "claiming.signer.tls.keyStorePassword",
    schemaField: "keyStorePassword",
    defaultKind: "placeholder:",
    required: false,
    secret: true,
    condition: "included with TLS object",
  },
  {
    section: "signer",
    expand: "prefix",
    envSuffix: "WEB3_SIGNER_TLS_TRUSTSTORE_PATH",
    field: "claiming.signer.tls.trustStorePath",
    schemaField: "trustStorePath",
    defaultKind: "placeholder:",
    required: false,
    secret: true,
    condition: "included with TLS object",
  },
  {
    section: "signer",
    expand: "prefix",
    envSuffix: "WEB3_SIGNER_TLS_TRUSTSTORE_PASSWORD",
    field: "claiming.signer.tls.trustStorePassword",
    schemaField: "trustStorePassword",
    defaultKind: "placeholder:",
    required: false,
    secret: true,
    condition: "included with TLS object",
  },
  {
    section: "signer",
    expand: "prefix",
    envSuffix: "AWS_KMS_KEY_ID",
    field: "claiming.signer.kmsKeyId",
    schemaField: "kmsKeyId",
    defaultKind: "placeholder:",
    required: false,
    secret: true,
    condition: 'used when SIGNER_TYPE is "aws-kms"',
  },
  {
    section: "signer",
    expand: "prefix",
    envSuffix: "AWS_KMS_REGION",
    field: "claiming.signer.region",
    schemaField: "region",
    defaultKind: "conditional",
    condition: "included only when set (aws-kms)",
    required: false,
    secret: true,
  },

  // database — passthrough leaves (no leaf .describe)
  {
    section: "database",
    env: "POSTGRES_HOST",
    field: "databaseOptions.host",
    schemaField: null,
    defaultKind: "literal:127.0.0.1",
    required: false,
    secret: true,
  },
  {
    section: "database",
    env: "POSTGRES_PORT",
    field: "databaseOptions.port",
    schemaField: null,
    defaultKind: "literal:5432",
    typeHint: "number",
    required: false,
  },
  {
    section: "database",
    env: "POSTGRES_USER",
    field: "databaseOptions.username",
    schemaField: null,
    defaultKind: "literal:postgres",
    required: false,
    secret: true,
  },
  {
    section: "database",
    env: "POSTGRES_PASSWORD",
    field: "databaseOptions.password",
    schemaField: null,
    defaultKind: "literal:postgres",
    required: false,
    secret: true,
  },
  {
    section: "database",
    env: "POSTGRES_DB",
    field: "databaseOptions.database",
    schemaField: null,
    defaultKind: "literal:postman_db",
    required: false,
  },
  {
    section: "database",
    env: "POSTGRES_SSL",
    field: "databaseOptions.ssl",
    schemaField: null,
    defaultKind: "bool-false",
    typeHint: "boolean",
    required: false,
    condition: 'when "true", ssl object is included',
  },
  {
    section: "database",
    env: "POSTGRES_SSL_REJECT_UNAUTHORIZED",
    field: "databaseOptions.ssl.rejectUnauthorized",
    schemaField: null,
    defaultKind: "bool-false",
    typeHint: "boolean",
    required: false,
    condition: "only when POSTGRES_SSL=true",
  },
  {
    section: "database",
    env: "POSTGRES_SSL_CA_PATH",
    field: "databaseOptions.ssl.ca",
    schemaField: null,
    defaultKind: "optional",
    required: false,
    secret: true,
    condition: "only when POSTGRES_SSL=true",
  },

  // database-cleaner
  {
    section: "database-cleaner",
    env: "DB_CLEANER_ENABLED",
    field: "databaseCleanerOptions.enabled",
    schemaField: "enabled",
    defaultKind: "bool-false",
    required: true,
  },
  {
    section: "database-cleaner",
    env: "DB_CLEANING_INTERVAL",
    field: "databaseCleanerOptions.cleaningInterval",
    schemaField: "cleaningInterval",
    defaultKind: "optional",
    required: false,
  },
  {
    section: "database-cleaner",
    env: "DB_DAYS_BEFORE_NOW_TO_DELETE",
    field: "databaseCleanerOptions.daysBeforeNowToDelete",
    schemaField: "daysBeforeNowToDelete",
    defaultKind: "optional",
    required: false,
  },

  // api
  {
    section: "api",
    env: "API_PORT",
    field: "apiOptions.port",
    schemaField: "port",
    defaultKind: "optional",
    required: false,
  },
];

function resolveDefault(defaultKind) {
  if (!defaultKind) return { value: null, placeholder: false };
  if (defaultKind === "bool-false") return { value: "false", placeholder: false };
  if (defaultKind === "optional" || defaultKind === "conditional") return { value: null, placeholder: false };
  if (defaultKind.startsWith("literal:")) {
    return { value: defaultKind.slice("literal:".length), placeholder: false };
  }
  if (defaultKind.startsWith("placeholder:")) {
    const raw = defaultKind.slice("placeholder:".length);
    return { value: null, placeholder: true, placeholderRaw: raw };
  }
  return { value: null, placeholder: false };
}

function expandInventoryEntry(entry) {
  if (entry.expand === "prefix") {
    return ["L1", "L2"].map((prefix) => {
      const expanded = {
        ...entry,
        env: `${prefix}_${entry.envSuffix}`,
        field: entry.field,
        expand: undefined,
        envSuffix: undefined,
        prefix,
      };
      if (typeof entry.condition === "string" && entry.condition.includes("SIGNER_TYPE")) {
        expanded.condition = entry.condition.replaceAll("SIGNER_TYPE", `${prefix}_SIGNER_TYPE`);
      }
      return expanded;
    });
  }
  if (entry.expand === "sponsoring") {
    // L1 claiming reads L2_L1_…; L2 claiming reads L1_L2_…
    return [
      {
        ...entry,
        env: "L2_L1_ENABLE_POSTMAN_SPONSORING",
        field: "l1Options.claiming.isPostmanSponsorshipEnabled",
        expand: undefined,
        prefix: "L1",
      },
      {
        ...entry,
        env: "L1_L2_ENABLE_POSTMAN_SPONSORING",
        field: "l2Options.claiming.isPostmanSponsorshipEnabled",
        expand: undefined,
        prefix: "L2",
      },
    ];
  }
  return [entry];
}

/**
 * Verify inventory defaultKind claims against envLoader.ts source text.
 * Only literal: and bool-false are checked; placeholder/optional/conditional skip.
 * Returns a list of mismatch messages (empty = OK).
 */
function verifyDefaultKinds(envLoaderSource, expandedEntries) {
  const mismatches = [];
  for (const entry of expandedEntries) {
    const kind = entry.defaultKind;
    if (!kind) continue;

    if (kind.startsWith("literal:")) {
      const value = kind.slice("literal:".length);
      const escaped = value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
      let matched = false;
      if (entry.prefix) {
        const suffix = entry.env.replace(new RegExp(`^${entry.prefix}_`), "");
        const tplRe = new RegExp(`process\\.env\\[\`\\$\\{prefix\\}_${suffix}\`\\]\\s*\\?\\?\\s*['"]${escaped}['"]`);
        matched = tplRe.test(envLoaderSource);
      } else {
        const dotRe = new RegExp(`process\\.env\\.${entry.env}\\s*\\?\\?\\s*['"]${escaped}['"]`);
        matched = dotRe.test(envLoaderSource);
      }
      if (!matched) {
        mismatches.push(
          `${entry.env}: defaultKind ${JSON.stringify(kind)} not found as ?? ${JSON.stringify(value)} in envLoader`,
        );
      }
      continue;
    }

    if (kind === "bool-false") {
      let matched = false;
      if (entry.prefix && entry.env.endsWith("_ENABLE_POSTMAN_SPONSORING")) {
        // Sponsoring uses ${opposite}_${prefix}_ENABLE_POSTMAN_SPONSORING
        const re = /process\.env\[`\$\{opposite\}_\$\{prefix\}_ENABLE_POSTMAN_SPONSORING`\]\s*===\s*["']true["']/;
        matched = re.test(envLoaderSource);
      } else if (entry.prefix) {
        const suffix = entry.env.replace(new RegExp(`^${entry.prefix}_`), "");
        const tplRe = new RegExp(`process\\.env\\[\`\\$\\{prefix\\}_${suffix}\`\\]\\s*===\\s*['"]true['"]`);
        matched = tplRe.test(envLoaderSource);
      } else {
        const dotRe = new RegExp(`process\\.env\\.${entry.env}\\s*===\\s*['"]true['"]`);
        matched = dotRe.test(envLoaderSource);
      }
      if (!matched) {
        mismatches.push(`${entry.env}: defaultKind "bool-false" not found as === "true" in envLoader`);
      }
    }
  }
  return mismatches;
}

/** Parse a JS/TS string literal starting at `pos` (must point at opening quote). */
function scanStringLiteral(src, pos) {
  const q = src[pos];
  if (q !== '"' && q !== "'") return null;
  let p = pos + 1;
  let out = "";
  while (p < src.length) {
    const c = src[p];
    if (c === "\\") {
      out += c + (src[p + 1] || "");
      p += 2;
      continue;
    }
    if (c === q) return { value: out, end: p + 1 };
    out += c;
    p++;
  }
  return null;
}

/**
 * Parse schema.ts for field → { description, type, optional, oneof }.
 * Finds each .describe("...") with a string-aware scanner (handles quotes inside strings).
 */
function parseSchemaFields(schemaSource) {
  const fields = new Map();
  const starts = [];
  for (let i = 0; (i = schemaSource.indexOf(".describe(", i)) !== -1; i += 10) {
    starts.push(i);
  }

  for (const start of starts) {
    let p = start + ".describe(".length;
    while (p < schemaSource.length && /\s/.test(schemaSource[p])) p++;
    const scanned = scanStringLiteral(schemaSource, p);
    if (!scanned) continue;
    const description = scanned.value.replace(/\s+/g, " ").trim();

    const before = schemaSource.slice(Math.max(0, start - 1500), start);
    const propRe = /(?:^|[\n{,])\s*([A-Za-z_][\w]*)\s*:/g;
    let last = null;
    let pm;
    while ((pm = propRe.exec(before)) !== null) last = pm;
    // Also allow `const name =` helper schemas
    if (!last) {
      const constRe = /(?:^|\n)\s*(?:export\s+)?const\s+([A-Za-z_][\w]*)\s*=/g;
      while ((pm = constRe.exec(before)) !== null) last = pm;
    }
    if (!last) continue;

    const name = last[1];
    const chain = before.slice(last.index) + schemaSource.slice(start, scanned.end);
    const optional = /\.optional\(/.test(chain) || /z\.any\(\)\.optional/.test(chain);
    const type = inferTypeFromChain(chain, name);

    const existing = fields.get(name);
    // Prefer property describes over earlier helper/parent describes when names collide
    if (!existing || (existing.description && description && chain.length < (existing.rawChain?.length || 0))) {
      // keep existing if it looks more specific (shorter chain = closer property)
    }
    if (!existing) {
      fields.set(name, { description, type, optional, rawChain: chain.slice(-300) });
    } else {
      // Overwrite when this describe is attached to a closer (shorter) property chain
      if (chain.length <= (existing.rawChain?.length || Infinity) + 50) {
        fields.set(name, { description, type, optional, rawChain: chain.slice(-300) });
      }
    }
  }

  // `privateKey: privateKeySchema` — map helper describe
  if (!fields.has("privateKey") || !fields.get("privateKey").description) {
    const helperStart = schemaSource.indexOf("privateKeySchema");
    if (helperStart >= 0) {
      const d = schemaSource.indexOf(".describe(", helperStart);
      if (d > 0 && d < helperStart + 400) {
        let p = d + ".describe(".length;
        while (/\s/.test(schemaSource[p])) p++;
        const scanned = scanStringLiteral(schemaSource, p);
        if (scanned) {
          fields.set("privateKey", {
            description: scanned.value.replace(/\s+/g, " ").trim(),
            type: "hex private key (32 bytes)",
            optional: false,
          });
        }
      }
    }
  }

  // Signer type literals — store under signerType to avoid colliding with postgres `type`
  const signerOneof = [];
  const signerDescs = [];
  for (const lit of schemaSource.matchAll(
    /z\.literal\(\s*["']([^"']+)["']\s*\)\.describe\(\s*(["'])(Signer type[\s\S]*?)\2\s*\)/g,
  )) {
    signerOneof.push(lit[1]);
    signerDescs.push(lit[3].replace(/\s+/g, " ").trim());
  }
  if (signerOneof.length) {
    fields.set("signerType", {
      description: 'Signer backend type: "private-key" (local key), "web3signer" (remote), or "aws-kms"',
      type: `string (${signerOneof.join("|")})`,
      optional: false,
      oneof: signerOneof,
      variantDescriptions: signerDescs,
    });
  }

  return fields;
}

function inferTypeFromChain(chain, fieldName) {
  // Strip whitespace so `z\n.number()` still matches `z.number(`
  const c = chain.replace(/\s+/g, "");
  if (/z\.boolean\(/.test(c)) return "boolean";
  if (/z\.bigint\(/.test(c)) {
    const mods = [];
    if (/\.positive\(/.test(c)) mods.push("positive");
    return mods.length ? `bigint (${mods.join(", ")})` : "bigint";
  }
  if (/z\.number\(/.test(c)) {
    const mods = [];
    if (/\.int\(/.test(c)) mods.push("int");
    if (/\.positive\(/.test(c)) mods.push("positive");
    if (/\.nonnegative\(/.test(c)) mods.push("nonnegative");
    const min = c.match(/\.min\((\d+)\)/);
    const max = c.match(/\.max\((\d+)\)/);
    if (min) mods.push(`min ${min[1]}`);
    if (max) mods.push(`max ${max[1]}`);
    return mods.length ? `number (${mods.join(", ")})` : "number";
  }
  if (/\.url\(/.test(c)) return "string (url)";
  if (/ethAddress/.test(c)) return "address";
  if (/hexString/.test(c)) return "hex string";
  if (/privateKeySchema/.test(c)) return "hex private key (32 bytes)";
  if (/z\.string\(/.test(c)) return "string";
  if (/z\.literal\(/.test(c)) {
    const lit = c.match(/z\.literal\(["']([^"']+)["']/);
    return lit ? `literal (${lit[1]})` : "literal";
  }
  if (/z\.any\(/.test(c)) return "any";
  if (/z\.object\(/.test(c) || /Schema/.test(c)) return "object";
  if (fieldName === "type") return "string";
  return "";
}

/**
 * Collect every concrete env var name referenced in envLoader.ts (after expansion awareness).
 * Used for completeness: inventory must cover all source reads.
 */
function collectSourceEnvRefs(envLoaderSource) {
  const refs = new Set();
  for (const m of envLoaderSource.matchAll(/process\.env\.([A-Z][A-Z0-9_]*)/g)) {
    refs.add(m[1]);
  }
  // Template forms: `${prefix}_FOO`, `${opposite}_${prefix}_FOO`
  for (const m of envLoaderSource.matchAll(/process\.env\[`\$\{prefix\}_([A-Z0-9_]+)`\]/g)) {
    refs.add(`L1_${m[1]}`);
    refs.add(`L2_${m[1]}`);
  }
  for (const m of envLoaderSource.matchAll(/process\.env\[`\$\{opposite\}_\$\{prefix\}_([A-Z0-9_]+)`\]/g)) {
    refs.add(`L1_L2_${m[1]}`);
    refs.add(`L2_L1_${m[1]}`);
  }
  return refs;
}

function formatRequired(entry, schemaMeta) {
  const parts = [];
  if (entry.required) parts.push("required");
  else parts.push("optional");
  if (entry.oneof && entry.oneof.length) {
    parts.push(`allowed: ${entry.oneof.join("|")}`);
  } else if (schemaMeta?.oneof?.length) {
    parts.push(`allowed: ${schemaMeta.oneof.join("|")}`);
  }
  if (entry.condition) parts.push(entry.condition);
  return parts.join("; ");
}

function isSecretEnv(env, flagged) {
  if (flagged) return true;
  return /(RPC_URL|PRIVATE_KEY|ENDPOINT|PASSWORD|AWS_KMS|CONTRACT_ADDRESS|PUBLIC_KEY|KEYSTORE|TRUSTSTORE|CLAIM_VIA|EVENT_FILTER_.*ADDRESS|POSTGRES_HOST|POSTGRES_USER|POSTGRES_SSL_CA)/.test(
    env,
  );
}

function extract(monorepoRoot) {
  const root = path.resolve(monorepoRoot);
  const envLoaderPath = path.join(root, ENV_LOADER_REL);
  const schemaPath = path.join(root, SCHEMA_REL);
  const envLoaderSource = fs.readFileSync(envLoaderPath, "utf8");
  const schemaSource = fs.readFileSync(schemaPath, "utf8");

  const schemaFields = parseSchemaFields(schemaSource);
  const sourceRefs = collectSourceEnvRefs(envLoaderSource);

  const expandedEntries = ENV_INVENTORY.flatMap((raw) => expandInventoryEntry(raw));
  const defaultKindMismatches = verifyDefaultKinds(envLoaderSource, expandedEntries);

  const rows = [];
  const missingDescriptions = [];
  const placeholderDefaults = [];
  const secretVars = [];
  const uncorrelated = [];

  for (const entry of expandedEntries) {
    const schemaMeta = entry.schemaField ? schemaFields.get(entry.schemaField) : null;
    let { value: defaultValue, placeholder, placeholderRaw } = resolveDefault(entry.defaultKind);

    let description = entry.description || schemaMeta?.description || "";

    let type = entry.typeHint || schemaMeta?.type || "";
    if (entry.oneof) {
      type = `string (${entry.oneof.join("|")})`;
    }

    const correlated = Boolean(description) || entry.schemaField === null;
    // passthrough DB leaves intentionally have no leaf describe
    if (!description) {
      missingDescriptions.push({
        envVar: entry.env,
        field: entry.field,
        schemaField: entry.schemaField,
      });
      if (entry.schemaField && !schemaMeta) {
        uncorrelated.push({ envVar: entry.env, field: entry.field, schemaField: entry.schemaField });
      }
    }

    if (placeholder) {
      placeholderDefaults.push({
        envVar: entry.env,
        placeholder: placeholderRaw ?? "",
      });
    }

    const secret = isSecretEnv(entry.env, entry.secret);
    if (secret) {
      secretVars.push(entry.env);
      // Never publish defaults for secrets/endpoints (even literal source fallbacks).
      if (defaultValue != null) {
        placeholderDefaults.push({
          envVar: entry.env,
          placeholder: String(defaultValue),
          reason: "secret-default-suppressed",
        });
        defaultValue = null;
      }
    }

    const requiredFlag =
      entry.required === true ? true : entry.required === false ? false : schemaMeta ? !schemaMeta.optional : false;

    rows.push({
      key: entry.env,
      envVar: entry.env,
      field: entry.field,
      description,
      default: defaultValue,
      type,
      required: requiredFlag,
      requiredLabel: formatRequired({ ...entry, required: requiredFlag }, schemaMeta),
      section: entry.section,
      secret,
      oneof: entry.oneof || schemaMeta?.oneof || null,
      defaultResolved: defaultValue != null,
      correlated,
    });
  }

  // Completeness vs source: every source ref must appear in inventory
  const inventoryEnvs = new Set(rows.map((r) => r.envVar));
  const missingFromInventory = [...sourceRefs].filter((e) => !inventoryEnvs.has(e)).sort();
  const extraInInventory = [...inventoryEnvs].filter((e) => !sourceRefs.has(e)).sort();

  rows.sort((a, b) => a.envVar.localeCompare(b.envVar));

  const perSection = {};
  for (const s of SECTION_META) {
    const sectionRows = rows.filter((r) => r.section === s.id);
    perSection[s.id] = {
      title: s.title,
      keyCount: sectionRows.length,
      noteOnly: false,
    };
  }

  const manifest = {
    generatedFrom: "postman/src/application/postman/app/config (static envLoader.ts ↔ schema.ts parse)",
    note:
      "Public-safe environment variable reference. Defaults are literal fallbacks from envLoader.ts only; " +
      'placeholders ("" / "0x") are left blank. Descriptions from Zod .describe() on schema.ts fields. ' +
      "Never reads .env or .env.sample.",
    counts: {
      total: rows.length,
      sections: SECTION_META.length,
      rendered: rows.length,
      excluded: 0,
    },
    perSection,
    sections: SECTION_META.map((s) => ({
      id: s.id,
      title: s.title,
      noteOnly: false,
      partial: `${s.id}.mdx`,
    })),
    keys: rows.map((r) => ({
      key: r.envVar,
      envVar: r.envVar,
      field: r.field,
      type: r.type,
      description: r.description,
      required: r.required,
      requiredLabel: r.requiredLabel,
      oneof: r.oneof,
      section: r.section,
      default: r.default,
      defaultResolved: r.defaultResolved,
      secret: r.secret,
      // prover-compat alias used by older render helpers
      allowedRequired: r.requiredLabel,
    })),
  };

  const report = {
    missingDescriptions,
    placeholderDefaults,
    secretVars: [...new Set(secretVars)].sort(),
    uncorrelated,
    schemaOnlyNoEnv: [
      {
        field: "claiming.feeRecipientAddress",
        reason: "Present in schema.ts with .describe() but no envLoader.ts mapping",
      },
    ],
    inventoryGaps: {
      missingFromInventory,
      extraInInventory,
    },
    defaultKindMismatches,
    sourceFiles: [ENV_LOADER_REL, SCHEMA_REL],
  };

  if (missingFromInventory.length || extraInInventory.length) {
    const err = new Error(
      `Env inventory drift vs ${ENV_LOADER_REL}: ` +
        `missing=${JSON.stringify(missingFromInventory)} extra=${JSON.stringify(extraInInventory)}`,
    );
    err.report = report;
    throw err;
  }

  if (defaultKindMismatches.length) {
    const err = new Error(
      `Env inventory defaultKind drift vs ${ENV_LOADER_REL}: ` + JSON.stringify(defaultKindMismatches),
    );
    err.report = report;
    throw err;
  }

  return { manifest, report };
}

module.exports = {
  extract,
  SECTION_META,
  ENV_INVENTORY,
  ENV_LOADER_REL,
  SCHEMA_REL,
  parseSchemaFields,
  collectSourceEnvRefs,
  expandInventoryEntry,
  resolveDefault,
  verifyDefaultKinds,
  PLACEHOLDER_DEFAULTS,
};
