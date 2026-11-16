import cKzg from "c-kzg";
import path from "path";
import { type Kzg, defineKzg } from "viem";

let kzg: Kzg | undefined;

/**
 * Idempotent KZG trusted setup initialization.
 * Safe to call from multiple test files — only runs once per process.
 * Returns the viem KZG interface (backed by `c-kzg`) used to build blob transactions.
 */
export function ensureKzgSetup(): Kzg {
  if (kzg === undefined) {
    const trustedSetupPath = path.resolve(import.meta.dirname, "../../_testData/trusted_setup.txt");
    cKzg.loadTrustedSetup(0, trustedSetupPath);
    kzg = defineKzg({
      blobToKzgCommitment: cKzg.blobToKzgCommitment,
      computeBlobKzgProof: cKzg.computeBlobKzgProof,
    });
  }
  return kzg;
}
