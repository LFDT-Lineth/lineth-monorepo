# Offset-bucket Huffman code (backref addresses)

Code lengths for MSB-bucket coding of LZ backref offsets, as used by DEFLATE
(distance codes) and zstd (offset codes): an offset is transmitted as an
entropy-coded bucket symbol `k` plus `k-1` raw bits, where `k` is the offset's
bit length. The bucket implies the leading 1, so the decoder reconstructs

    offset = (1 << (k-1)) | extra_bits

This replaces the v3 encoding of one near/far flag bit plus a raw 14- or 21-bit
address, and removes the near/far distinction from the format entirely.

Trained on the 15,631 backrefs in the first 780,000 bytes of
`2026-07-28_recent.payload.bin`, parsed from the lzss stream (the LZ parse, and
therefore the offsets, are identical between the deployed and v3 formats -- same
matcher, same input, same 64 KiB dictionary and 2 MiB window).

**Max code length is 9 bits**, so the decoder needs only a single-level
512-entry lookup table with no fallback path -- simpler than the length table,
whose 19-bit max requires a 12-bit table plus a canonical walk. That 9-bit
maximum is corpus-specific; length-limiting the code to 12 bits would cost a
negligible fraction of the gain and guarantee the single-level property
out-of-sample.

No minimum code length is required here (unlike the 512-symbol literal/length
table, where a min of 8 keeps trailing pad bits from completing a codeword):
an offset is only ever read after a length symbol has already been decoded, so
there is no end-of-stream ambiguity to protect against.


| bucket `k` | address range | count | share | code length | extra bits | total bits |
|---:|---|---:|---:|---:|---:|---:|
| 1 | 1 | 13 | 0.08% | 9 | 0 | 9 |
| 2 | 2–3 | 6 | 0.04% | 9 | 1 | 10 |
| 3 | 4–7 | 137 | 0.88% | 7 | 2 | 9 |
| 4 | 8–15 | 94 | 0.60% | 8 | 3 | 11 |
| 5 | 16–31 | 179 | 1.15% | 6 | 4 | 10 |
| 6 | 32–63 | 450 | 2.88% | 5 | 5 | 10 |
| 7 | 64–127 | 452 | 2.89% | 5 | 6 | 11 |
| 8 | 128–255 | 495 | 3.17% | 5 | 7 | 12 |
| 9 | 256–511 | 707 | 4.52% | 5 | 8 | 13 |
| 10 | 512–1,023 | 824 | 5.27% | 4 | 9 | 13 |
| 11 | 1,024–2,047 | 764 | 4.89% | 4 | 10 | 14 |
| 12 | 2,048–4,095 | 1,014 | 6.49% | 4 | 11 | 15 |
| 13 | 4,096–8,191 | 1,524 | 9.75% | 3 | 12 | 15 |
| 14 | 8,192–16,383 | 2,428 | 15.53% | 3 | 13 | 16 |
| 15 | 16,384–32,767 | 937 | 5.99% | 4 | 14 | 18 |
| 16 | 32,768–65,535 | 1,219 | 7.80% | 4 | 15 | 19 |
| 17 | 65,536–131,071 | 943 | 6.03% | 4 | 16 | 20 |
| 18 | 131,072–262,143 | 1,333 | 8.53% | 4 | 17 | 21 |
| 19 | 262,144–524,287 | 1,545 | 9.88% | 3 | 18 | 21 |
| 20 | 524,288–1,048,575 | 567 | 3.63% | 5 | 19 | 24 |

Totals over 15,631 backrefs:

| | bits | per backref |
|---|---:|---:|
| bucket symbols | 60,938 | 3.899 |
| raw extra bits | 199,158 | 12.741 |
| **total** | **260,096** | **16.640** |
| v3 (flag + raw 14/21) | 280,259 | 17.930 |

Saving: **2,520 bytes = 1.20%** of the 210,040-byte v3 stream.
The Shannon bound on the bucket symbols is 3.865 bits, so this code is within
0.034 bits of optimal.

To be folded into the `compress` CLI as a table-generation mode.

