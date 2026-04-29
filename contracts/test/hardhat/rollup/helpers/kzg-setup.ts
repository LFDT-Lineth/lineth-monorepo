import * as kzg from "c-kzg";
import path from "path";

let initialized = false;

/**
 * Idempotent KZG trusted setup initialization.
 * Safe to call from multiple test files — only runs once per process.
 */
export function ensureKzgSetup(): void {
  if (initialized) return;
  const trustedSetupPath = path.resolve(__dirname, "../../_testData/trusted_setup.txt");
  kzg.loadTrustedSetup(0, trustedSetupPath);
  initialized = true;
}
