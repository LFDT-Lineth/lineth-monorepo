import { BitString, ObjectIdentifier, Sequence, verifySchema } from "asn1js";
import { Hex } from "viem";

/**
 * Extracts the 65-byte uncompressed secp256k1 public key ({@code 0x04 || x || y}) from a
 * DER-encoded SubjectPublicKeyInfo structure (RFC 5280 §4.1, RFC 5480 §2).
 *
 * Uses `asn1js` to walk the ASN.1 schema:
 * ```asn1
 * SubjectPublicKeyInfo ::= SEQUENCE {
 *   algorithm  AlgorithmIdentifier,  -- SEQUENCE { OID ecPublicKey, OID secp256k1 }
 *   subjectPublicKey  BIT STRING      -- 0x04 || x(32) || y(32)
 * }
 * ```
 *
 * @param {Uint8Array} derBytes - The raw DER-encoded SubjectPublicKeyInfo bytes (typically 88 bytes for secp256k1).
 * @returns {Hex} The uncompressed public key as a hex string (`"0x04..."`, 130 hex chars).
 * @throws {Error} If the ASN.1 structure cannot be parsed or the key is not a 65-byte uncompressed point.
 */
export function extractUncompressedPublicKeyFromDer(derBytes: Uint8Array): Hex {
  const schema = new Sequence({
    value: [new Sequence({ value: [new ObjectIdentifier()] }), new BitString({ name: "subjectPublicKey" })],
  });

  const parsed = verifySchema(derBytes, schema);
  if (!parsed.verified) {
    throw new Error("Failed to parse DER-encoded SubjectPublicKeyInfo");
  }

  const keyBytes = new Uint8Array(parsed.result.subjectPublicKey.valueBlock.valueHexView);
  if (keyBytes.length !== 65 || keyBytes[0] !== 0x04) {
    throw new Error("Expected 65-byte uncompressed secp256k1 public key (0x04 prefix)");
  }

  return `0x${Buffer.from(keyBytes).toString("hex")}` as Hex;
}
