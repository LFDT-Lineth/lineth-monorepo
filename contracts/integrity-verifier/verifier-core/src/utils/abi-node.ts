/**
 * Contract Integrity Verifier - ABI Node.js Utilities
 *
 * Node.js-only functions that use the filesystem.
 * These are separated to avoid bundling 'fs' in browser builds.
 */

import { readFileSync } from "fs";

import { parseArtifact } from "./abi";

import type { NormalizedArtifact } from "../types";

/**
 * Loads and normalizes an artifact file (Hardhat or Foundry).
 * Node.js only - uses filesystem.
 *
 * @throws Error with descriptive message if file cannot be read or parsed
 */
export function loadArtifact(filePath: string): NormalizedArtifact {
  let content: string;
  try {
    content = readFileSync(filePath, "utf-8");
  } catch (err) {
    throw new Error(`Failed to read artifact file at ${filePath}: ${err instanceof Error ? err.message : String(err)}`);
  }

  return parseArtifact(content, filePath);
}
