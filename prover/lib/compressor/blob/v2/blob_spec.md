# Linea Blob Format Specification

This document provides a detailed explanation of the structure of a blob in the context of Linea prover. A blob contains essential information that allows the Linea prover to validate the execution of several blocks of transactions. The blob's content is compressed and structured to facilitate cryptographic operations within zkSNARK circuits.

## Content

At its core, the blob stores a list of batches. Each batch can contain multiple blocks. However, not all block information is included, as the prover can infer elements like signatures or chainID. Instead, the blob includes the block timestamp and the list of RLP encoded transactions.

## Layout

The blob consists of two main parts: the Header (uncompressed) and the Body (compressed).

```
|------------------ Blob -------------------|
| Header (uncompressed) | Body (compressed) |
```

### Header

The header contains the following elements:

```
|----------------------------------- Header ----------------------------------|
| Blob Version | Dictionary Checksum | Number of Batches |  Batch Lengths...  |
|--------------|---------------------|-------------------|--------------------|
|    2 bytes   |       32 bytes      |      2 bytes      |        ...         |

|- Batch Length (in bytes) -|
|---------------------------|
|         3 bytes           |
```

- Blob Version (2 bytes): Currently 0xfffe, big endian, counting down.
- Dictionary Checksum (32 bytes): Used for the compression dictionary checksum.
- Number of Batches (2 bytes uint16, big endian): Indicates the number of batches.
- For each batch:
  - Number of bytes in batch (3 bytes uint24, big endian): Length of the batch in bytes.

### Body

The body contains a list of RLP encoded Linea blocks. For each block, the raw data includes:

- Block Timestamp uint64
- Transactions (refer to EncodeTxForCompression for more details)

The raw data is compressed using [compress/lzss](https://github.com/consensys/compress), a snark-friendly compression algorithm. This allows the Linea prover to "prove correct decompression of the blob".

### Final Blob: Byte Alignment

The header and the compressed body are concatenated. The resulting `[]byte` slice is packed into field elements. Due to hashing requirements and blob content verification, we can't fully utilize the 4096*32bytes; each 32byte is valid only if it is a valid field element.

The bls12-381 scalar field bit size is 255; hence, we can only use up to 254 bits out of the 256bits (in the 32bytes) in practice.

This packing operation may add some extra padding bytes to the blob.

See `blob.PackAlign` and `blob.UnpackAlign` for more info.

## Reference implementation

We provide `DecompressBlob(data, dictionary)` that decompresses the body and builds a `Blob` data structure in: TODO @gbotrel add link once open sourced.
