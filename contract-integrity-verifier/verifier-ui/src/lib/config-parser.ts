import type { VerifierConfig } from "@consensys/linea-contract-integrity-verifier";
import type { ParsedConfig, FileRef, ConfigFormat, FieldType, FormField } from "@/types";

// ============================================================================
// Environment Variable Extraction
// ============================================================================

const ENV_VAR_REGEX = /\$\{([^}]+)\}/g;

export function extractEnvVars(content: string): string[] {
  const matches = content.matchAll(ENV_VAR_REGEX);
  const vars = new Set<string>();

  for (const match of matches) {
    vars.add(match[1]);
  }

  return Array.from(vars).sort();
}

// ============================================================================
// JSON Pre-processing for Env Vars
// ============================================================================

/**
 * Replaces environment variable placeholders with valid JSON values.
 * This allows parsing JSON files that contain ${VAR} syntax.
 *
 * - Unquoted ${VAR} (likely numbers) -> 0
 * - Quoted "${VAR}" -> "__PLACEHOLDER__{VAR}__"
 * - Inside arrays as params -> "__PLACEHOLDER__{VAR}__"
 */
export function preprocessJsonWithEnvVars(content: string): string {
  // First, handle unquoted env vars (typically used for numbers like chainId)
  // Pattern: key: ${VAR} or key:${VAR} (not inside quotes)
  let processed = content.replace(/:\s*\$\{([^}]+)\}(\s*[,}\]])/g, ": 0$2");

  // Handle quoted env vars - replace with placeholder strings
  // Pattern: "${VAR}" -> "__PLACEHOLDER__VAR__"
  processed = processed.replace(/"\$\{([^}]+)\}"/g, '"__PLACEHOLDER__$1__"');

  return processed;
}

// ============================================================================
// File Reference Extraction
// ============================================================================

export function extractFileReferences(config: VerifierConfig): FileRef[] {
  const files: FileRef[] = [];

  for (const contract of config.contracts) {
    // Artifact file is always required (skip if it's a placeholder)
    if (contract.artifactFile && !contract.artifactFile.startsWith("__PLACEHOLDER__")) {
      files.push({
        path: contract.artifactFile,
        type: "artifact",
        contractName: contract.name,
        uploaded: false,
      });
    }

    // Schema file if state verification is configured
    if (
      contract.stateVerification?.schemaFile &&
      !contract.stateVerification.schemaFile.startsWith("__PLACEHOLDER__")
    ) {
      files.push({
        path: contract.stateVerification.schemaFile,
        type: "schema",
        contractName: contract.name,
        uploaded: false,
      });
    }
  }

  // Dedupe by path (multiple contracts might share files)
  const seen = new Set<string>();
  return files.filter((file) => {
    if (seen.has(file.path)) {
      return false;
    }
    seen.add(file.path);
    return true;
  });
}

// ============================================================================
// Markdown Config Parsing
// ============================================================================

interface MarkdownBlock {
  type: "chain" | "contract" | "state";
  name: string;
  content: Record<string, string>;
  stateTable?: Array<Record<string, string>>;
}

function parseMarkdownBlocks(content: string): MarkdownBlock[] {
  const blocks: MarkdownBlock[] = [];
  const lines = content.split("\n");

  let currentBlock: MarkdownBlock | null = null;
  let inCodeBlock = false;
  let codeContent: string[] = [];

  for (const line of lines) {
    // Check for headers
    const chainMatch = line.match(/^##\s+Chain:\s*(.+)/i);
    const contractMatch = line.match(/^##\s+Contract:\s*(.+)/i);

    if (chainMatch) {
      currentBlock = { type: "chain", name: chainMatch[1].trim(), content: {} };
      blocks.push(currentBlock);
      continue;
    }

    if (contractMatch) {
      currentBlock = { type: "contract", name: contractMatch[1].trim(), content: {} };
      blocks.push(currentBlock);
      continue;
    }

    // Check for code blocks
    if (line.startsWith("```verifier") || line.startsWith("```yaml")) {
      inCodeBlock = true;
      codeContent = [];
      continue;
    }

    if (line.startsWith("```") && inCodeBlock) {
      inCodeBlock = false;
      if (currentBlock) {
        for (const codeLine of codeContent) {
          const [key, ...valueParts] = codeLine.split(":");
          if (key && valueParts.length > 0) {
            currentBlock.content[key.trim()] = valueParts.join(":").trim();
          }
        }
      }
      continue;
    }

    if (inCodeBlock) {
      codeContent.push(line);
    }
  }

  return blocks;
}

export function parseMarkdownConfig(content: string): VerifierConfig {
  const blocks = parseMarkdownBlocks(content);

  const chains: Record<string, { chainId: number; rpcUrl: string; explorerUrl?: string }> = {};
  const contracts: VerifierConfig["contracts"] = [];

  for (const block of blocks) {
    if (block.type === "chain") {
      chains[block.name] = {
        chainId: parseInt(block.content.chainId || "0", 10),
        rpcUrl: block.content.rpcUrl || block.content.rpc || "",
        explorerUrl: block.content.explorerUrl || block.content.explorer,
      };
    }

    if (block.type === "contract") {
      const contract: VerifierConfig["contracts"][0] = {
        name: block.content.name || block.name,
        chain: block.content.chain || "",
        address: block.content.address || "",
        artifactFile: block.content.artifact || block.content.artifactFile || "",
        isProxy: block.content.isProxy === "true",
      };

      if (block.content.schema || block.content.schemaFile) {
        contract.stateVerification = {
          schemaFile: block.content.schema || block.content.schemaFile,
          ozVersion: (block.content.ozVersion as "v4" | "v5" | "auto") || "auto",
        };
      }

      contracts.push(contract);
    }
  }

  return { chains, contracts };
}

// ============================================================================
// Validation Schema (relaxed for placeholder values)
// ============================================================================

/**
 * Relaxed validation that accepts placeholder values.
 * Full validation happens after env var interpolation.
 */
function validateConfigStructure(config: unknown): config is VerifierConfig {
  if (typeof config !== "object" || config === null) return false;

  const c = config as Record<string, unknown>;

  // Must have chains object
  if (typeof c.chains !== "object" || c.chains === null) return false;

  // Must have contracts array
  if (!Array.isArray(c.contracts)) return false;

  // Each contract must have required fields (can be placeholders)
  for (const contract of c.contracts) {
    if (typeof contract !== "object" || contract === null) return false;
    const ct = contract as Record<string, unknown>;
    if (typeof ct.name !== "string") return false;
    if (typeof ct.chain !== "string") return false;
    if (typeof ct.address !== "string") return false;
    if (typeof ct.artifactFile !== "string") return false;
  }

  return true;
}

// ============================================================================
// Config Parser
// ============================================================================

export function parseConfig(content: string, filename: string): ParsedConfig {
  const format: ConfigFormat = filename.endsWith(".md") ? "markdown" : "json";

  // Extract env vars from original content first
  const envVars = extractEnvVars(content);

  let raw: VerifierConfig;

  if (format === "markdown") {
    raw = parseMarkdownConfig(content);
  } else {
    // Preprocess JSON to handle env var placeholders
    const preprocessed = preprocessJsonWithEnvVars(content);
    try {
      raw = JSON.parse(preprocessed) as VerifierConfig;
    } catch (e) {
      throw new Error(`Invalid JSON: ${e instanceof Error ? e.message : "Parse error"}`);
    }
  }

  // Relaxed validation (accepts placeholders)
  if (!validateConfigStructure(raw)) {
    throw new Error("Invalid config structure: missing required fields (chains, contracts)");
  }

  const requiredFiles = extractFileReferences(raw);

  return {
    raw,
    rawContent: content, // Store original content for later interpolation
    filename,
    format,
    envVars,
    requiredFiles,
    chains: Object.keys(raw.chains),
    contracts: raw.contracts.map((c) => c.name),
  };
}

// ============================================================================
// Form Field Generation
// ============================================================================

export function detectFieldType(varName: string): FieldType {
  const name = varName.toUpperCase();

  if (name.includes("RPC") || name.includes("URL") || name.includes("ENDPOINT")) {
    return "url";
  }

  if (name.includes("ADDRESS") || name.includes("ADDR")) {
    return "address";
  }

  if (name.includes("KEY") || name.includes("SECRET") || name.includes("PASSWORD")) {
    return "password";
  }

  if (name.includes("CHAIN_ID") || name.includes("CHAINID") || name.includes("BLOCK") || name.includes("PORT")) {
    return "number";
  }

  return "text";
}

export function humanizeVarName(varName: string): string {
  return varName
    .replace(/_/g, " ")
    .replace(/([a-z])([A-Z])/g, "$1 $2")
    .split(" ")
    .map((word) => word.charAt(0).toUpperCase() + word.slice(1).toLowerCase())
    .join(" ");
}

export function generateFormFields(envVars: string[]): FormField[] {
  return envVars.map((name) => {
    const type = detectFieldType(name);

    return {
      name,
      type,
      label: humanizeVarName(name),
      placeholder: getPlaceholder(type, name),
      required: true,
    };
  });
}

function getPlaceholder(type: FieldType, varName: string): string {
  switch (type) {
    case "url":
      return "https://...";
    case "address":
      return "0x...";
    case "number":
      return "0";
    case "password":
      return "••••••••";
    default:
      return `Enter ${humanizeVarName(varName).toLowerCase()}`;
  }
}

// ============================================================================
// Config Rewriting
// ============================================================================

/**
 * Deep clone an object, with fallback for older browsers that don't support structuredClone.
 */
function deepClone<T>(obj: T): T {
  if (typeof structuredClone === "function") {
    return structuredClone(obj);
  }
  // Fallback for older browsers (Safari < 15.4)
  return JSON.parse(JSON.stringify(obj));
}

export function rewriteConfigPaths(config: VerifierConfig, fileMap: Record<string, string>): VerifierConfig {
  const rewritten = deepClone(config);

  for (const contract of rewritten.contracts) {
    // Rewrite artifact path
    if (fileMap[contract.artifactFile]) {
      contract.artifactFile = fileMap[contract.artifactFile];
    }

    // Rewrite schema path
    if (contract.stateVerification?.schemaFile) {
      const originalPath = contract.stateVerification.schemaFile;
      if (fileMap[originalPath]) {
        contract.stateVerification.schemaFile = fileMap[originalPath];
      }
    }
  }

  return rewritten;
}

/**
 * Interpolates environment variables in the raw config content.
 * This is the proper way to handle configs with env var placeholders.
 */
export function interpolateEnvVarsInContent(content: string, envVars: Record<string, string>): string {
  return content.replace(ENV_VAR_REGEX, (match, varName) => {
    const value = envVars[varName];
    if (value === undefined) {
      throw new Error(`Missing environment variable: ${varName}`);
    }
    return value;
  });
}
