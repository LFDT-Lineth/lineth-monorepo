import { extractUncompressedPublicKeyFromDer } from "../ec-publickey";

describe("extractUncompressedPublicKeyFromDer", () => {
  // secp256k1 public key = point (x, y) on the curve, each coordinate is 32 bytes → 64 bytes total
  const PUBLIC_KEY_XY = new Uint8Array(64).fill(0xab);

  // SEC 1 §2.3.3 uncompressed point encoding: 0x04 prefix || x(32) || y(32) = 65 bytes
  // 0x04 = uncompressed, 0x02/0x03 = compressed (x + parity bit of y)
  const UNCOMPRESSED_KEY = new Uint8Array([0x04, ...PUBLIC_KEY_XY]);

  // DER-encoded SubjectPublicKeyInfo (RFC 5280 §4.1, RFC 5480 §2) — 23-byte header:
  //
  // SubjectPublicKeyInfo ::= SEQUENCE {
  //   algorithm  AlgorithmIdentifier,   -- identifies key type + curve
  //   subjectPublicKey  BIT STRING      -- the actual EC point
  // }
  //
  // Byte-level breakdown:
  //   30 56             — SEQUENCE tag (0x30), length 86 bytes (0x56) — outer container
  //     30 10           — SEQUENCE tag, length 16 — AlgorithmIdentifier
  //       06 07 2a 86 48 ce 3d 02 01
  //                     — OID tag (0x06), 7 bytes: 1.2.840.10045.2.1 (id-ecPublicKey, ANSI X9.62)
  //       06 05 2b 81 04 00 0a
  //                     — OID tag (0x06), 5 bytes: 1.3.132.0.10 (secp256k1, SEC 2 / CERTICOM)
  //     03 42 00        — BIT STRING tag (0x03), length 66 (0x42), 0 unused trailing bits
  //                       payload = 65-byte uncompressed EC point (0x04 || x || y)
  const DER_HEADER = new Uint8Array([
    0x30, 0x56, 0x30, 0x10, 0x06, 0x07, 0x2a, 0x86, 0x48, 0xce, 0x3d, 0x02, 0x01, 0x06, 0x05, 0x2b, 0x81, 0x04, 0x00,
    0x0a, 0x03, 0x42, 0x00,
  ]);

  // Full 88-byte DER structure: 23-byte header + 65-byte uncompressed key
  const DER_PUBLIC_KEY = new Uint8Array([...DER_HEADER, ...UNCOMPRESSED_KEY]);

  it("should extract 65-byte uncompressed key from DER-encoded SubjectPublicKeyInfo", () => {
    const result = extractUncompressedPublicKeyFromDer(DER_PUBLIC_KEY);

    expect(result).toBe(`0x04${"ab".repeat(64)}`);
  });

  it("should throw when ASN.1 structure is invalid", () => {
    const garbage = new Uint8Array(88).fill(0xff);

    expect(() => extractUncompressedPublicKeyFromDer(garbage)).toThrow(
      "Failed to parse DER-encoded SubjectPublicKeyInfo",
    );
  });

  it("should throw when key does not start with 0x04 prefix (compressed key)", () => {
    // Swap 0x04 (uncompressed) for 0x02 (compressed, even y) at the start of the key payload
    const badDer = new Uint8Array(DER_PUBLIC_KEY);
    badDer[DER_HEADER.length] = 0x02;

    expect(() => extractUncompressedPublicKeyFromDer(badDer)).toThrow(
      "Expected 65-byte uncompressed secp256k1 public key (0x04 prefix)",
    );
  });

  it("should throw when key length is wrong", () => {
    // Construct a valid SubjectPublicKeyInfo that wraps a 33-byte compressed key instead of 65-byte
    // uncompressed. The ASN.1 parses fine, but the extracted key fails the length/prefix check.
    const compressedKey = new Uint8Array([0x02, ...new Uint8Array(32).fill(0xab)]);
    // BIT STRING: tag 0x03, length 34 (1 unused-bits byte + 33 key bytes), 0x00 unused bits
    const shortBitString = new Uint8Array([0x03, 0x22, 0x00, ...compressedKey]);
    const shortDer = new Uint8Array([
      // Outer SEQUENCE: tag 0x30, length = AlgorithmIdentifier(16) + BIT STRING
      0x30,
      16 + shortBitString.length,
      // AlgorithmIdentifier SEQUENCE (same OIDs as above)
      0x30,
      0x10,
      0x06,
      0x07,
      0x2a,
      0x86,
      0x48,
      0xce,
      0x3d,
      0x02,
      0x01,
      0x06,
      0x05,
      0x2b,
      0x81,
      0x04,
      0x00,
      0x0a,
      ...shortBitString,
    ]);

    expect(() => extractUncompressedPublicKeyFromDer(shortDer)).toThrow(
      "Expected 65-byte uncompressed secp256k1 public key (0x04 prefix)",
    );
  });
});
