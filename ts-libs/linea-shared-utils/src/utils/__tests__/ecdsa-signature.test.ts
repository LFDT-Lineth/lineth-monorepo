import { Address, Hex, recoverAddress } from "viem";

import { bigintToHex32, decodeDerSignature, recoverYParity } from "../ecdsa-signature";

jest.mock("viem", () => {
  const actual = jest.requireActual("viem");
  return {
    ...actual,
    recoverAddress: jest.fn(),
  };
});

const recoverAddressMock = recoverAddress as jest.MockedFunction<typeof recoverAddress>;

describe("decodeDerSignature", () => {
  // DER-encoded ECDSA-Sig-Value (RFC 3279 §2.2.3):
  //
  // ECDSA-Sig-Value ::= SEQUENCE { r INTEGER, s INTEGER }
  //
  // Byte-level breakdown:
  //   30 44       — SEQUENCE tag (0x30), length 68 bytes (0x44)
  //     02 20     — INTEGER tag (0x02), length 32 (0x20) → r component
  //     <r: 32 bytes>
  //     02 20     — INTEGER tag (0x02), length 32 (0x20) → s component
  //     <s: 32 bytes>
  //
  // This is the format AWS KMS returns from the Sign API (ECDSA_SHA_256).
  const R_BYTES = new Uint8Array(32).fill(0x01);
  const S_BYTES = new Uint8Array(32).fill(0x02);
  const DER_SIGNATURE = new Uint8Array([0x30, 0x44, 0x02, 0x20, ...R_BYTES, 0x02, 0x20, ...S_BYTES]);

  it("should parse DER-encoded ECDSA signature into r and s", () => {
    const { r, s } = decodeDerSignature(DER_SIGNATURE);

    expect(r).toBe(BigInt(`0x${"01".repeat(32)}`));
    expect(s).toBe(BigInt(`0x${"02".repeat(32)}`));
  });

  it("should handle DER integers with leading zero padding", () => {
    // DER INTEGER is a signed type. If the high bit of the value is set (>= 0x80),
    // a leading 0x00 byte is prepended to keep the value positive.
    //   r = 0x80 → high bit set → DER encoding: 02 02 00 80 (2 value bytes)
    //   s = 0x7f → high bit clear → DER encoding: 02 01 7f   (1 value byte)
    const derSig = new Uint8Array([0x30, 0x07, 0x02, 0x02, 0x00, 0x80, 0x02, 0x01, 0x7f]);

    const { r, s } = decodeDerSignature(derSig);

    expect(r).toBe(0x80n);
    expect(s).toBe(0x7fn);
  });

  it("should normalize s to lower half of curve order (EIP-2)", () => {
    // EIP-2 requires s ≤ secp256k1_N / 2 to prevent signature malleability.
    // For any valid (r, s) signature, (r, N - s) is also valid — EIP-2 removes
    // this ambiguity by mandating the lower-half value.
    // Here s = 0xffff...ff which is above half-N, so it must be flipped to N - s.
    const highSBytes = new Uint8Array(32).fill(0xff);
    const highSDerSig = new Uint8Array([0x30, 0x44, 0x02, 0x20, ...R_BYTES, 0x02, 0x20, ...highSBytes]);

    const { s } = decodeDerSignature(highSDerSig);

    const originalS = BigInt(`0x${"ff".repeat(32)}`);
    expect(s).not.toBe(originalS);
    const SECP256K1_HALF_N = 0xfffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd0364141n / 2n;
    expect(s <= SECP256K1_HALF_N).toBe(true);
  });

  it("should not change s when already in lower half", () => {
    // s = 0x020202...02 is well below half-N (~3.6e37 vs ~5.8e37), no normalisation needed
    const { s } = decodeDerSignature(DER_SIGNATURE);

    expect(s).toBe(BigInt(`0x${"02".repeat(32)}`));
  });

  it("should throw when DER structure is invalid", () => {
    expect(() => decodeDerSignature(new Uint8Array([0xff, 0x00]))).toThrow(
      "Failed to parse DER-encoded ECDSA signature",
    );
  });
});

// yParity (aka recovery id, v) selects which of the two possible EC points produced
// the signature. Given (r, s), there are two candidate public keys (corresponding to the
// positive and negative Y coordinates of the curve point with X = r). The correct one is
// the one that recovers to the known signer address. We try yParity=0, then yParity=1.
describe("recoverYParity", () => {
  const HASH: Hex = "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef";
  const R: Hex = `0x${"01".repeat(32)}`;
  const S: Hex = `0x${"02".repeat(32)}`;
  const ADDRESS: Address = "0xD42E308FC964b71E18126dF469c21B0d7bcb86cC";

  beforeEach(() => {
    jest.clearAllMocks();
  });

  it("should return 0 when yParity=0 recovers the correct address", async () => {
    recoverAddressMock.mockResolvedValueOnce(ADDRESS);

    const result = await recoverYParity(HASH, R, S, ADDRESS);

    expect(result).toBe(0);
    expect(recoverAddressMock).toHaveBeenCalledTimes(1);
  });

  it("should return 1 when yParity=0 does not match but yParity=1 does", async () => {
    recoverAddressMock
      .mockResolvedValueOnce("0x0000000000000000000000000000000000000001" as Address)
      .mockResolvedValueOnce(ADDRESS);

    const result = await recoverYParity(HASH, R, S, ADDRESS);

    expect(result).toBe(1);
    expect(recoverAddressMock).toHaveBeenCalledTimes(2);
  });

  it("should throw when neither candidate recovers the address", async () => {
    recoverAddressMock.mockResolvedValue("0x0000000000000000000000000000000000000001" as Address);

    await expect(recoverYParity(HASH, R, S, ADDRESS)).rejects.toThrow(
      "Failed to determine signature recovery parameter (yParity)",
    );
  });

  it("should handle recoverAddress throwing for one candidate", async () => {
    // Some (r, s, yParity) combos produce an invalid point — recoverAddress will throw.
    // The function should swallow the error and try the other parity.
    recoverAddressMock.mockRejectedValueOnce(new Error("invalid")).mockResolvedValueOnce(ADDRESS);

    const result = await recoverYParity(HASH, R, S, ADDRESS);

    expect(result).toBe(1);
  });

  it("should preserve the last recoverAddress error as the thrown error's cause", async () => {
    // When both yParity candidates throw, the caller loses all diagnostic signal.
    // The function must re-throw with the last underlying error attached as `cause`
    // so callers can surface the real failure.
    const firstErr = new Error("invalid point for yParity=0");
    const secondErr = new Error("invalid point for yParity=1");
    recoverAddressMock.mockRejectedValueOnce(firstErr).mockRejectedValueOnce(secondErr);

    const thrown = await recoverYParity(HASH, R, S, ADDRESS).catch((e: Error) => e);

    expect(thrown).toBeInstanceOf(Error);
    expect((thrown as Error).message).toContain("Failed to determine signature recovery parameter (yParity)");
    expect((thrown as Error).cause).toBe(secondErr);
  });

  it("should perform case-insensitive address comparison", async () => {
    // EIP-55 mixed-case checksums mean the same address can appear in different casings
    recoverAddressMock.mockResolvedValueOnce(ADDRESS.toLowerCase() as Address);

    const result = await recoverYParity(HASH, R, S, ADDRESS);

    expect(result).toBe(0);
  });
});

// Ethereum signatures expect r and s as fixed-width 32-byte (64 hex char) strings.
// BigInt.toString(16) omits leading zeros, so we pad to 64 chars.
describe("bigintToHex32", () => {
  it("should convert a small value to zero-padded 32-byte hex", () => {
    // 1n → "0x0000...0001" (63 zeros + "1")
    expect(bigintToHex32(1n)).toBe(`0x${"0".repeat(63)}1`);
  });

  it("should convert a 32-byte value without extra padding", () => {
    // Already fills all 64 hex chars — no padding needed
    const value = BigInt(`0x${"ff".repeat(32)}`);
    expect(bigintToHex32(value)).toBe(`0x${"ff".repeat(32)}`);
  });
});
