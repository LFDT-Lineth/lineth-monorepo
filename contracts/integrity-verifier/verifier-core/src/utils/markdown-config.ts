/**
 * Contract Integrity Verifier - Markdown Config Parser
 *
 * Parses markdown files as verification configs.
 * Allows documentation to serve as the config source of truth.
 *
 * This file is browser-compatible - uses no Node.js-only modules.
 */

import { MAX_MARKDOWN_CONFIG_SIZE } from "../constants";
import {
  VerifierConfig,
  ChainConfig,
  ContractConfig,
  StateVerificationConfig,
  ViewCallConfig,
  SlotConfig,
  StoragePathConfig,
} from "../types";

interface ParsedContract {
  name: string;
  address: string;
  chain: string;
  artifact: string;
  isProxy: boolean;
  ozVersion: "v4" | "v5" | "auto" | undefined;
  schema: string | undefined;
  viewCalls: ViewCallConfig[];
  slots: SlotConfig[];
  storagePaths: StoragePathConfig[];
}

/**
 * Browser-safe path join function.
 * Joins path segments, handling both Unix and Windows separators.
 */
function joinPath(base: string, relative: string): string {
  // If relative is absolute, return it
  if (relative.startsWith("/") || /^[a-zA-Z]:/.test(relative)) {
    return relative;
  }
  // Normalize base path (remove trailing slashes)
  // Use trimEnd approach to avoid ReDoS with nested quantifiers
  let normalizedBase = base;
  while (normalizedBase.endsWith("/") || normalizedBase.endsWith("\\")) {
    normalizedBase = normalizedBase.slice(0, -1);
  }
  // Join with forward slash
  return normalizedBase + "/" + relative;
}

/**
 * Parse a verifier metadata block (YAML-like key: value pairs)
 */
function parseVerifierBlock(block: string): Record<string, string> {
  const result: Record<string, string> = {};
  const lines = block.trim().split("\n");

  for (const line of lines) {
    const colonIndex = line.indexOf(":");
    if (colonIndex > 0) {
      const key = line.slice(0, colonIndex).trim();
      const value = line.slice(colonIndex + 1).trim();
      result[key] = value;
    }
  }

  return result;
}

/**
 * Parse a markdown table into rows of cells
 * Preserves empty cells to maintain column alignment
 */
function parseMarkdownTable(tableText: string): string[][] {
  const lines = tableText.trim().split("\n");
  const rows: string[][] = [];

  for (const line of lines) {
    // Skip empty lines
    if (!line.trim()) continue;

    // Skip separator lines (e.g., |---|---|---| or | --- | --- |)
    // A separator line consists only of |, -, :, and whitespace
    if (/^\s*\|[\s|:-]+\|\s*$/.test(line) && line.includes("---")) continue;

    // Parse cells - split by | and trim each cell
    // Remove first and last elements if empty (from leading/trailing |)
    const rawCells = line.split("|").map((cell) => cell.trim());

    // Remove leading empty string (before first |)
    if (rawCells.length > 0 && rawCells[0] === "") {
      rawCells.shift();
    }
    // Remove trailing empty string (after last |)
    if (rawCells.length > 0 && rawCells[rawCells.length - 1] === "") {
      rawCells.pop();
    }

    if (rawCells.length > 0) {
      rows.push(rawCells);
    }
  }

  return rows;
}

/**
 * Find markdown tables containing a "Type" column header.
 * Uses line-by-line parsing to avoid ReDoS vulnerabilities.
 */
function findMarkdownTables(text: string): string[] {
  const tables: string[] = [];
  const lines = text.split("\n");

  let i = 0;
  while (i < lines.length) {
    const line = lines[i];

    // Look for a header row containing "Type" (case-insensitive)
    if (line.includes("|") && /type/i.test(line)) {
      // Check if the next line is a separator (contains |, -, and optionally : and spaces)
      const nextLine = lines[i + 1];
      if (nextLine && /^\s*\|[\s|:-]+\|\s*$/.test(nextLine) && nextLine.includes("-")) {
        // Found a table header + separator, collect all rows
        const tableLines: string[] = [line, nextLine];
        let j = i + 2;

        // Collect body rows (lines starting with |)
        while (j < lines.length && lines[j].trim().startsWith("|") && lines[j].includes("|")) {
          tableLines.push(lines[j]);
          j++;
        }

        if (tableLines.length > 2) {
          tables.push(tableLines.join("\n"));
        }

        i = j;
        continue;
      }
    }

    i++;
  }

  return tables;
}

/**
 * Parse a value string into the appropriate type
 */
function parseValue(value: string): unknown {
  const trimmed = value.trim();

  // Boolean
  if (trimmed === "true") return true;
  if (trimmed === "false") return false;

  // Number (but not hex addresses)
  if (/^-?\d+$/.test(trimmed) && !trimmed.startsWith("0x")) {
    return trimmed; // Keep as string for large numbers
  }

  // Remove backticks if present
  if (trimmed.startsWith("`") && trimmed.endsWith("`")) {
    return trimmed.slice(1, -1);
  }

  return trimmed;
}

/**
 * Parse comma-separated parameters
 */
function parseParams(paramsStr: string): unknown[] | undefined {
  if (!paramsStr || paramsStr.trim() === "") return undefined;

  return paramsStr.split(",").map((p) => parseValue(p.trim()));
}

/**
 * Parse a verification check row from a markdown table
 */
function parseCheckRow(
  row: string[],
  headers: string[],
): { type: string; check: ViewCallConfig | SlotConfig | StoragePathConfig } | null {
  // Find column indices
  const typeIdx = headers.findIndex((h) => h.toLowerCase() === "type");
  const descIdx = headers.findIndex(
    (h) => h.toLowerCase().includes("description") || h.toLowerCase().includes("comment"),
  );
  const checkIdx = headers.findIndex(
    (h) => h.toLowerCase() === "check" || h.toLowerCase() === "function" || h.toLowerCase() === "path",
  );
  const paramsIdx = headers.findIndex((h) => h.toLowerCase() === "params" || h.toLowerCase() === "type/params");
  const expectedIdx = headers.findIndex((h) => h.toLowerCase() === "expected");

  if (typeIdx === -1 || checkIdx === -1 || expectedIdx === -1) {
    return null;
  }

  const type = row[typeIdx]?.toLowerCase().trim();
  const check = row[checkIdx]?.trim();
  const params = row[paramsIdx]?.trim() || "";
  const expected = parseValue(row[expectedIdx] || "");
  const description = descIdx >= 0 ? row[descIdx]?.trim() : undefined;

  if (!type || !check) return null;

  if (type === "viewcall" || type === "view") {
    const viewCall: ViewCallConfig = {
      function: check.replace(/`/g, ""),
      expected,
    };
    const parsedParams = parseParams(params.replace(/`/g, ""));
    if (parsedParams && parsedParams.length > 0) {
      viewCall.params = parsedParams;
    }
    return { type: "viewCall", check: viewCall };
  }

  if (type === "slot") {
    // For slots, params column contains the type
    const rawSlotType = params.replace(/`/g, "").toLowerCase();

    // Validate slot type - use uint256 as default for unknown types
    // Accepts: address, bool, uint8-uint256 (8-bit increments), int8-int256, bytes1-bytes32
    const slotType: SlotConfig["type"] = isValidSlotType(rawSlotType) ? (rawSlotType as SlotConfig["type"]) : "uint256";

    const slot: SlotConfig = {
      slot: check.replace(/`/g, ""),
      type: slotType,
      name: description || check,
      expected,
    };
    return { type: "slot", check: slot };
  }

  if (type === "storagepath" || type === "path") {
    const storagePath: StoragePathConfig = {
      path: check.replace(/`/g, ""),
      expected,
    };
    return { type: "storagePath", check: storagePath };
  }

  return null;
}

/**
 * Validates an Ethereum address format.
 */
function isValidAddress(address: string): boolean {
  return /^0x[a-fA-F0-9]{40}$/.test(address);
}

/**
 * Validates if a string is a valid Solidity slot type.
 * Accepts: address, bool, uint8-uint256 (8-bit increments), int8-int256, bytes1-bytes32
 */
function isValidSlotType(type: string): boolean {
  // Fixed types
  if (type === "address" || type === "bool") {
    return true;
  }

  // uint<N> where N is 8-256 in multiples of 8
  const uintMatch = type.match(/^uint(\d+)$/);
  if (uintMatch) {
    const bits = parseInt(uintMatch[1], 10);
    return bits >= 8 && bits <= 256 && bits % 8 === 0;
  }

  // int<N> where N is 8-256 in multiples of 8
  const intMatch = type.match(/^int(\d+)$/);
  if (intMatch) {
    const bits = parseInt(intMatch[1], 10);
    return bits >= 8 && bits <= 256 && bits % 8 === 0;
  }

  // bytes<N> where N is 1-32
  const bytesMatch = type.match(/^bytes(\d+)$/);
  if (bytesMatch) {
    const bytes = parseInt(bytesMatch[1], 10);
    return bytes >= 1 && bytes <= 32;
  }

  return false;
}

/**
 * Extract chains configuration from markdown
 * Looks for a ```chains block or uses defaults
 */
function extractChains(markdown: string): Record<string, ChainConfig> {
  const chainsMatch = markdown.match(/```chains\n([\s\S]*?)```/);

  if (chainsMatch) {
    try {
      const parsed = JSON.parse(chainsMatch[1]);
      // Validate that parsed is an object with chain configs
      if (typeof parsed === "object" && parsed !== null && !Array.isArray(parsed)) {
        return parsed as Record<string, ChainConfig>;
      }
      console.warn("Warning: Invalid chains block format in markdown, using defaults");
    } catch (err) {
      console.warn(
        `Warning: Failed to parse chains block in markdown: ${err instanceof Error ? err.message : String(err)}`,
      );
      // Fall through to defaults
    }
  }

  // Default chains if not specified
  return {
    "ethereum-mainnet": {
      chainId: 1,
      rpcUrl: "${ETHEREUM_MAINNET_RPC_URL}",
      explorerUrl: "https://etherscan.io",
    },
    "ethereum-sepolia": {
      chainId: 11155111,
      rpcUrl: "${ETHEREUM_SEPOLIA_RPC_URL}",
      explorerUrl: "https://sepolia.etherscan.io",
    },
    "linea-mainnet": {
      chainId: 59144,
      rpcUrl: "${LINEA_MAINNET_RPC_URL}",
      explorerUrl: "https://lineascan.build",
    },
    "linea-sepolia": {
      chainId: 59141,
      rpcUrl: "${LINEA_SEPOLIA_RPC_URL}",
      explorerUrl: "https://sepolia.lineascan.build",
    },
  };
}

/**
 * Parse a markdown file into a VerifierConfig.
 * Includes size validation to prevent DoS from large files.
 */
export function parseMarkdownConfig(markdown: string, configDir: string): VerifierConfig {
  // Validate size to prevent DoS
  if (markdown.length > MAX_MARKDOWN_CONFIG_SIZE) {
    throw new Error(`Markdown content exceeds maximum size of ${MAX_MARKDOWN_CONFIG_SIZE / 1024 / 1024}MB`);
  }

  const contracts: ContractConfig[] = [];
  const chains = extractChains(markdown);

  // Find all contract sections
  // A contract section starts with ## Contract: Name or a ```verifier block
  const sections = markdown.split(/(?=##\s+Contract:)|(?=```verifier)/);

  let currentContract: ParsedContract | null = null;
  let lastContractHeader: string | null = null;

  for (const section of sections) {
    // Check for contract header (## Contract: Name)
    const headerMatch = section.match(/^##\s+Contract:\s*([^\n]+)/);
    if (headerMatch) {
      lastContractHeader = headerMatch[1].trim();
    }

    // Check for verifier block
    const verifierMatch = section.match(/```verifier\n([\s\S]*?)```/);

    if (verifierMatch) {
      const metadata = parseVerifierBlock(verifierMatch[1]);

      // Extract contract name from metadata, current section header, or previous header
      const nameMatch = section.match(/##\s+Contract:\s*([^\n]+)/);
      const name =
        metadata.name ||
        (nameMatch ? nameMatch[1].trim() : null) ||
        lastContractHeader ||
        `Contract-${contracts.length + 1}`;

      // Validate address if provided
      const address = metadata.address || "";
      if (address && !isValidAddress(address)) {
        console.warn(`Warning: Invalid address format for contract '${name}': ${address}`);
      }

      currentContract = {
        name,
        address,
        chain: metadata.chain || "ethereum-mainnet",
        artifact: metadata.artifact || metadata.artifactFile || "",
        isProxy: metadata.isProxy === "true",
        ozVersion: (metadata.ozVersion as "v4" | "v5" | "auto") || undefined,
        schema: metadata.schema || metadata.schemaFile || undefined,
        viewCalls: [],
        slots: [],
        storagePaths: [],
      };
    }

    // Find verification tables in this section using line-by-line parsing
    // This avoids ReDoS vulnerabilities from complex regex patterns
    const tables = findMarkdownTables(section);

    for (const fullTable of tables) {
      if (!currentContract) continue;

      const rows = parseMarkdownTable(fullTable);

      if (rows.length < 2) continue;

      const headers = rows[0];

      for (let i = 1; i < rows.length; i++) {
        const parsed = parseCheckRow(rows[i], headers);
        if (parsed) {
          if (parsed.type === "viewCall") {
            currentContract.viewCalls.push(parsed.check as ViewCallConfig);
          } else if (parsed.type === "slot") {
            currentContract.slots.push(parsed.check as SlotConfig);
          } else if (parsed.type === "storagePath") {
            currentContract.storagePaths.push(parsed.check as StoragePathConfig);
          }
        }
      }
    }

    // If we have a contract with a valid address, add it to the list
    if (currentContract && currentContract.address && isValidAddress(currentContract.address)) {
      // Validate artifact path is not empty
      if (!currentContract.artifact) {
        console.warn(`Warning: Contract '${currentContract.name}' has no artifact path, skipping`);
        currentContract = null;
        continue;
      }

      const contractConfig: ContractConfig = {
        name: currentContract.name,
        chain: currentContract.chain,
        address: currentContract.address,
        artifactFile: joinPath(configDir, currentContract.artifact),
        isProxy: currentContract.isProxy,
      };

      // Add state verification if there are any checks
      if (
        currentContract.viewCalls.length > 0 ||
        currentContract.slots.length > 0 ||
        currentContract.storagePaths.length > 0
      ) {
        const stateVerification: StateVerificationConfig = {};

        if (currentContract.ozVersion) {
          stateVerification.ozVersion = currentContract.ozVersion;
        }
        if (currentContract.schema) {
          stateVerification.schemaFile = currentContract.schema;
        }
        if (currentContract.viewCalls.length > 0) {
          stateVerification.viewCalls = currentContract.viewCalls;
        }
        if (currentContract.slots.length > 0) {
          stateVerification.slots = currentContract.slots;
        }
        if (currentContract.storagePaths.length > 0) {
          stateVerification.storagePaths = currentContract.storagePaths;
        }

        contractConfig.stateVerification = stateVerification;
      }

      contracts.push(contractConfig);
      currentContract = null;
    }
  }

  return { chains, contracts };
}
