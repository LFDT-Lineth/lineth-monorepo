import { Integer, Sequence, verifySchema } from "asn1js";
import { Address, Hex, recoverAddress, type RecoverAddressErrorType } from "viem";

const SECP256K1_N = 0xfffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd0364141n;
const SECP256K1_HALF_N = SECP256K1_N / 2n;

/**
 * Parses a DER-encoded ECDSA-Sig-Value (RFC 3279 §2.2.3) into r and s integer components,
 * normalising s to the lower half of the secp256k1 curve order per EIP-2.
 *
 * Uses `asn1js` to walk the ASN.1 schema:
 * ```asn1
 * ECDSA-Sig-Value ::= SEQUENCE { r INTEGER, s INTEGER }
 * ```
 *
 * DER INTEGERs may carry a leading 0x00 byte for positive-sign encoding; this is
 * handled transparently by BigInt hex parsing.
 *
 * @param {Uint8Array} derSignature - The raw DER-encoded ECDSA signature.
 * @returns {{ r: bigint; s: bigint }} The r and s components (s already normalised).
 * @throws {Error} If the ASN.1 structure cannot be parsed.
 */
export function decodeDerSignature(derSignature: Uint8Array): { r: bigint; s: bigint } {
  const schema = new Sequence({
    value: [new Integer({ name: "r" }), new Integer({ name: "s" })],
  });

  const parsed = verifySchema(derSignature, schema);
  if (!parsed.verified) {
    throw new Error("Failed to parse DER-encoded ECDSA signature");
  }

  const r = BigInt(`0x${Buffer.from(parsed.result.r.valueBlock.valueHexView).toString("hex")}`);
  let s = BigInt(`0x${Buffer.from(parsed.result.s.valueBlock.valueHexView).toString("hex")}`);

  if (s > SECP256K1_HALF_N) {
    s = SECP256K1_N - s;
  }

  return { r, s };
}

/**
 * Determines the yParity (recovery id) by trying both candidate values (0 and 1)
 * and checking which one recovers to the known signer address.
 *
 * Due to the nature of elliptic curves, two solutions exist for a given ECDSA
 * signature (corresponding to the positive and negative Y coordinates of the
 * curve point). The yParity selects which was used during signing.
 *
 * @param {Hex} hash - The keccak256 hash of the serialised transaction.
 * @param {Hex} r - The r component of the signature as a 32-byte hex string.
 * @param {Hex} s - The s component of the signature as a 32-byte hex string.
 * @param {Address} signerAddress - The expected signer address.
 * @returns {Promise<number>} The recovery id (0 or 1).
 * @throws {Error} If neither candidate recovers the expected address.
 */
export async function recoverYParity(hash: Hex, r: Hex, s: Hex, signerAddress: Address): Promise<number> {
  // viem's recoverAddress/recoverPublicKey throws RecoverAddressErrorType (Error & { name }).
  // Upstream messages from viem and @noble/curves are value-free (generic crypto diagnostics
  // such as "invalid signature r: out of range" or "invalid recovery: point at infinity") and
  // do not embed r/s/hash values, so propagating as `cause` is safe from a logging perspective.
  let lastError: RecoverAddressErrorType | undefined;
  for (const yParity of [0, 1] as const) {
    try {
      const recoveredAddress = await recoverAddress({ hash, signature: { r, s, yParity } });
      if (recoveredAddress.toLowerCase() === signerAddress.toLowerCase()) {
        return yParity;
      }
    } catch (err) {
      lastError = err as RecoverAddressErrorType;
      continue;
    }
  }
  throw new Error(
    "Failed to determine signature recovery parameter (yParity)",
    lastError ? { cause: lastError } : undefined,
  );
}

/**
 * Converts a bigint to a 0x-prefixed, zero-padded 32-byte hex string.
 */
export function bigintToHex32(value: bigint): Hex {
  return `0x${value.toString(16).padStart(64, "0")}` as Hex;
}
