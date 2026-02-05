/**
 * Contract Integrity Verifier - Storage Node.js Utilities
 *
 * Node.js-only functions that use the filesystem.
 * These are separated to avoid bundling 'fs' in browser builds.
 */

import { readFileSync } from "fs";
import { resolve } from "path";

import { parseStorageSchema } from "./storage";

import type { StorageSchema } from "../types";

/**
 * Loads a storage schema from a JSON file.
 * Node.js only - uses filesystem.
 */
export function loadStorageSchema(schemaPath: string, configDir: string): StorageSchema {
  const resolvedPath = resolve(configDir, schemaPath);
  let content: string;
  try {
    content = readFileSync(resolvedPath, "utf-8");
  } catch (err) {
    throw new Error(
      `Failed to read schema file at ${resolvedPath}: ${err instanceof Error ? err.message : String(err)}`,
    );
  }

  return parseStorageSchema(content);
}
