import { ethers } from "ethers";
import fs from "fs";
import path from "path";

import { envVarNameFromContext, formatEnvVarValueForMessage, isEnvVarContext } from "./envVarLogging";

export const REGISTRY_DIR = path.join(__dirname, "..", "..", "deployments", "addresses");

export const REGISTRY_NETWORKS = new Set(["mainnet", "sepolia", "hoodi", "linea_mainnet", "linea_sepolia"]);

export const EXPECTED_CHAIN_IDS: Record<string, number> = {
  mainnet: 1,
  sepolia: 11155111,
  hoodi: 560048,
  linea_mainnet: 59144,
  linea_sepolia: 59141,
};

export interface AddressObject {
  address: string;
  notes?: string;
}

export interface SingleRegistryEntry {
  address: string;
  notes?: string;
}

export interface MultiRegistryEntry {
  addresses: AddressObject[];
  notes?: string;
}

export type RegistryContractEntry = SingleRegistryEntry | MultiRegistryEntry;

export interface NetworkRegistry {
  $schema?: string;
  network: string;
  chainId: number;
  contracts: Record<string, RegistryContractEntry>;
}

export interface RegistryValidationIssue {
  file: string;
  path: string;
  message: string;
}

interface DuplicateJsonKey {
  path: string;
  key: string;
}

export function isMultiRegistryEntry(entry: RegistryContractEntry): entry is MultiRegistryEntry {
  return "addresses" in entry && Array.isArray(entry.addresses);
}

export function parseChecksummedAddress(raw: string, context: string): string {
  if (!ethers.isAddress(raw)) {
    if (isEnvVarContext(context)) {
      const envVarName = envVarNameFromContext(context);
      throw new Error(
        `Environment variable "${envVarName}" is not a valid EVM address. Got: "${formatEnvVarValueForMessage(envVarName, raw)}". ` +
          `Expected a 0x-prefixed 40-hex-character address.`,
      );
    }
    throw new Error(`${context}: invalid EVM address "${raw}"`);
  }
  return ethers.getAddress(raw);
}

export function assertChecksummedAddress(address: string, context: string): void {
  const checksummed = parseChecksummedAddress(address, context);
  if (address !== checksummed) {
    throw new Error(`${context}: address must be EIP-55 checksummed. Expected "${checksummed}", got "${address}".`);
  }
}

export function resolveRegistryEntryAddresses(entry: RegistryContractEntry): string[] {
  if (isMultiRegistryEntry(entry)) {
    return entry.addresses.map((item, index) => parseChecksummedAddress(item.address, `addresses[${index}].address`));
  }
  return [parseChecksummedAddress(entry.address, "address")];
}

/**
 * Returns populated addresses for a registry entry, or `undefined` when the entry is
 * treated as unpopulated (single zero address, or an `addresses` array where every item is zero).
 */
export function getPopulatedAddresses(entry: RegistryContractEntry): string[] | undefined {
  const addresses = resolveRegistryEntryAddresses(entry);
  const nonZero = addresses.filter((value) => value !== ethers.ZeroAddress);
  if (nonZero.length === 0) {
    return undefined;
  }
  if (nonZero.length !== addresses.length) {
    throw new Error(
      "Registry entry mixes zero and non-zero addresses. Populate every item or use zero placeholders for the entire entry.",
    );
  }
  return addresses;
}

export function loadRegistry(networkName: string): NetworkRegistry | undefined {
  const filePath = path.join(REGISTRY_DIR, `${networkName}.json`);
  if (!fs.existsSync(filePath)) {
    return undefined;
  }
  const raw = fs.readFileSync(filePath, "utf-8");
  return JSON.parse(raw) as NetworkRegistry;
}

export function lookupRegistryEntry(
  registry: NetworkRegistry,
  contractKey: string,
  envVarName?: string,
): RegistryContractEntry | undefined {
  if (registry.contracts[contractKey] !== undefined) {
    return registry.contracts[contractKey];
  }
  if (envVarName !== undefined && envVarName !== contractKey && registry.contracts[envVarName] !== undefined) {
    return registry.contracts[envVarName];
  }
  return undefined;
}

export function parseEnvAddressList(envVarName: string): string[] | undefined {
  const raw = process.env[envVarName];
  if (!raw?.trim()) {
    return undefined;
  }
  return raw.split(",").map((part, index) => parseChecksummedAddress(part.trim(), `${envVarName}[${index}]`));
}

export function parseEnvAddress(envVarName: string): string | undefined {
  const addresses = parseEnvAddressList(envVarName);
  if (addresses === undefined) {
    return undefined;
  }
  if (addresses.length !== 1) {
    throw new Error(
      `Environment variable "${envVarName}" must contain exactly one address, got ${addresses.length}. ` +
        "Use the multi-address registry helper for comma-delimited address lists.",
    );
  }
  return addresses[0];
}

export function sameAddressSet(left: string[], right: string[]): boolean {
  if (left.length !== right.length) {
    return false;
  }
  const sortedLeft = [...left].sort();
  const sortedRight = [...right].sort();
  return sortedLeft.every((value, index) => value === sortedRight[index]);
}

function findDuplicateJsonKeys(source: string): DuplicateJsonKey[] {
  const duplicates: DuplicateJsonKey[] = [];
  let index = 0;

  const skipWhitespace = () => {
    while (/\s/.test(source[index] ?? "")) {
      index += 1;
    }
  };

  const parseString = (): string => {
    if (source[index] !== '"') {
      throw new Error(`Expected string at offset ${index}.`);
    }
    let value = "";
    index += 1;
    while (index < source.length) {
      const char = source[index];
      if (char === '"') {
        index += 1;
        return value;
      }
      if (char === "\\") {
        const escapeStart = index;
        index += 1;
        const escaped = source[index];
        if (escaped === "u") {
          const hex = source.slice(index + 1, index + 5);
          if (!/^[0-9a-fA-F]{4}$/.test(hex)) {
            throw new Error(`Invalid unicode escape at offset ${escapeStart}.`);
          }
          value += String.fromCharCode(parseInt(hex, 16));
          index += 5;
          continue;
        }
        const escapes: Record<string, string> = {
          '"': '"',
          "\\": "\\",
          "/": "/",
          b: "\b",
          f: "\f",
          n: "\n",
          r: "\r",
          t: "\t",
        };
        if (escaped === undefined || escapes[escaped] === undefined) {
          throw new Error(`Invalid escape at offset ${escapeStart}.`);
        }
        value += escapes[escaped];
        index += 1;
        continue;
      }
      value += char;
      index += 1;
    }
    throw new Error("Unterminated string.");
  };

  const parseValue = (pathParts: string[]): void => {
    skipWhitespace();
    const char = source[index];
    if (char === "{") {
      parseObject(pathParts);
      return;
    }
    if (char === "[") {
      parseArray(pathParts);
      return;
    }
    if (char === '"') {
      parseString();
      return;
    }
    while (index < source.length && !/[,\]}]/.test(source[index])) {
      index += 1;
    }
  };

  const parseArray = (pathParts: string[]): void => {
    index += 1;
    skipWhitespace();
    let itemIndex = 0;
    while (source[index] !== "]") {
      parseValue([...pathParts, `[${itemIndex}]`]);
      itemIndex += 1;
      skipWhitespace();
      if (source[index] === ",") {
        index += 1;
        skipWhitespace();
        continue;
      }
      if (source[index] !== "]") {
        throw new Error(`Expected "," or "]" at offset ${index}.`);
      }
    }
    index += 1;
  };

  const parseObject = (pathParts: string[]): void => {
    index += 1;
    skipWhitespace();
    const seen = new Set<string>();
    while (source[index] !== "}") {
      const key = parseString();
      const pathLabel = pathParts.length > 0 ? pathParts.join(".") : "$";
      if (seen.has(key)) {
        duplicates.push({ path: pathLabel, key });
      }
      seen.add(key);
      skipWhitespace();
      if (source[index] !== ":") {
        throw new Error(`Expected ":" at offset ${index}.`);
      }
      index += 1;
      parseValue([...pathParts, key]);
      skipWhitespace();
      if (source[index] === ",") {
        index += 1;
        skipWhitespace();
        continue;
      }
      if (source[index] !== "}") {
        throw new Error(`Expected "," or "}" at offset ${index}.`);
      }
    }
    index += 1;
  };

  skipWhitespace();
  parseValue([]);
  return duplicates;
}

function toIssueMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

function addIssue(issues: RegistryValidationIssue[], file: string, path: string, message: string): void {
  issues.push({ file, path, message });
}

function validateNotes(file: string, notesPath: string, notes: unknown, issues: RegistryValidationIssue[]): void {
  if (notes !== undefined && typeof notes !== "string") {
    addIssue(issues, file, notesPath, "`notes` must be a string when present.");
  }
}

function validateAllowedProperties(
  file: string,
  entryPath: string,
  record: Record<string, unknown>,
  allowedProperties: string[],
  issues: RegistryValidationIssue[],
): void {
  const allowed = new Set(allowedProperties);
  for (const property of Object.keys(record)) {
    if (!allowed.has(property)) {
      addIssue(issues, file, `${entryPath}.${property}`, `Unexpected property "${property}".`);
    }
  }
}

function validateAddressObject(
  file: string,
  entryPath: string,
  entry: unknown,
  issues: RegistryValidationIssue[],
): entry is AddressObject {
  if (entry === null || typeof entry !== "object" || Array.isArray(entry)) {
    addIssue(issues, file, entryPath, "Address item must be an object.");
    return false;
  }

  const record = entry as Record<string, unknown>;
  validateAllowedProperties(file, entryPath, record, ["address", "notes"], issues);
  if (typeof record.address !== "string") {
    addIssue(issues, file, `${entryPath}.address`, "`address` must be a string.");
    return false;
  }

  try {
    assertChecksummedAddress(record.address, `${file} ${entryPath}.address`);
  } catch (error) {
    addIssue(issues, file, `${entryPath}.address`, toIssueMessage(error));
  }

  validateNotes(file, `${entryPath}.notes`, record.notes, issues);
  return true;
}

function validateDuplicateAddressObjects(
  file: string,
  addressesPath: string,
  entries: unknown[],
  issues: RegistryValidationIssue[],
): void {
  const seen = new Set<string>();
  for (const entry of entries) {
    if (entry === null || typeof entry !== "object" || Array.isArray(entry)) {
      continue;
    }
    const address = (entry as Record<string, unknown>).address;
    if (typeof address !== "string" || !ethers.isAddress(address)) {
      continue;
    }
    const normalized = ethers.getAddress(address).toLowerCase();
    if (seen.has(normalized)) {
      addIssue(issues, file, addressesPath, `Duplicate address "${address}" in \`addresses\` array.`);
      return;
    }
    seen.add(normalized);
  }
}

function validateRegistryEntryShape(
  file: string,
  contractKey: string,
  entry: unknown,
  issues: RegistryValidationIssue[],
): entry is RegistryContractEntry {
  const entryPath = `contracts.${contractKey}`;
  const issueCountBefore = issues.length;

  if (entry === null || typeof entry !== "object" || Array.isArray(entry)) {
    addIssue(issues, file, entryPath, "Entry must be an object.");
    return false;
  }

  const record = entry as Record<string, unknown>;
  const hasAddress = typeof record.address === "string";
  const hasAddresses = Array.isArray(record.addresses);

  if (hasAddress && hasAddresses) {
    addIssue(issues, file, entryPath, "Entry must not define both `address` and `addresses`.");
    return false;
  }

  if (!hasAddress && !hasAddresses) {
    addIssue(issues, file, entryPath, "Entry must define either `address` or `addresses`.");
    return false;
  }

  if (hasAddresses) {
    const addressItems = record.addresses as unknown[];
    validateAllowedProperties(file, entryPath, record, ["addresses", "notes"], issues);
    if (addressItems.length === 0) {
      addIssue(issues, file, `${entryPath}.addresses`, "`addresses` must contain at least one item.");
      return false;
    }

    for (const [index, item] of addressItems.entries()) {
      validateAddressObject(file, `${entryPath}.addresses[${index}]`, item, issues);
    }

    validateDuplicateAddressObjects(file, `${entryPath}.addresses`, addressItems, issues);
    validateNotes(file, `${entryPath}.notes`, record.notes, issues);
  } else if (typeof record.address === "string") {
    validateAddressObject(file, entryPath, record, issues);
  } else {
    addIssue(issues, file, `${entryPath}.address`, "`address` must be a string.");
    return false;
  }

  return issues.length === issueCountBefore;
}

function validateMixedZeroPopulation(
  file: string,
  contractKey: string,
  entry: RegistryContractEntry,
  issues: RegistryValidationIssue[],
): void {
  try {
    getPopulatedAddresses(entry);
  } catch (error) {
    addIssue(issues, file, `contracts.${contractKey}`, toIssueMessage(error));
  }
}

export function validateRegistryFile(fileName: string): RegistryValidationIssue[] {
  const filePath = path.join(REGISTRY_DIR, fileName);
  const issues: RegistryValidationIssue[] = [];

  if (!fs.existsSync(filePath)) {
    addIssue(issues, fileName, fileName, "Registry file is missing.");
    return issues;
  }

  let registry: NetworkRegistry;
  const raw = fs.readFileSync(filePath, "utf-8");
  try {
    for (const duplicate of findDuplicateJsonKeys(raw)) {
      addIssue(
        issues,
        fileName,
        duplicate.path,
        `Duplicate JSON key "${duplicate.key}". JSON parsers keep only the last value.`,
      );
    }
    registry = JSON.parse(raw) as NetworkRegistry;
  } catch (error) {
    addIssue(issues, fileName, fileName, `Invalid JSON: ${toIssueMessage(error)}`);
    return issues;
  }

  const networkName = fileName.replace(/\.json$/, "");
  if (registry.network !== networkName) {
    addIssue(issues, fileName, "network", `Expected network "${networkName}", got "${registry.network}".`);
  }

  const expectedChainId = EXPECTED_CHAIN_IDS[networkName];
  if (expectedChainId === undefined) {
    addIssue(issues, fileName, "network", `Unknown registry network "${networkName}".`);
  } else if (registry.chainId !== expectedChainId) {
    addIssue(issues, fileName, "chainId", `Expected chainId ${expectedChainId}, got ${registry.chainId}.`);
  }

  if (registry.contracts === null || typeof registry.contracts !== "object" || Array.isArray(registry.contracts)) {
    addIssue(issues, fileName, "contracts", "`contracts` must be an object.");
    return issues;
  }

  for (const [contractKey, entry] of Object.entries(registry.contracts)) {
    if (validateRegistryEntryShape(fileName, contractKey, entry, issues)) {
      validateMixedZeroPopulation(fileName, contractKey, entry, issues);
    }
  }

  return issues;
}

export function validateAllAddressRegistries(): RegistryValidationIssue[] {
  return [...REGISTRY_NETWORKS].flatMap((networkName) => validateRegistryFile(`${networkName}.json`));
}
