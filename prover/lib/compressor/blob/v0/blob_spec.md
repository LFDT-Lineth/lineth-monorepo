# Lineth Blob Format Specification

This document provides a detailed explanation of the structure of a blob in the context of Lineth prover. A blob contains essential information that allows the Lineth prover to validate the execution of several blocks of transactions. The blob's content is compressed and structured to facilitate cryptographic operations within zkSNARK circuits.

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
|--------------------------- Header ---------------------------|
| Dictionary Checksum | Number of Batches |     Batches...     |
|---------------------|-------------------|--------------------|
|       32 bytes      |      2 bytes      |        ...         |

|--------------------------- Batch ----------------------------|
| Number of Blocks |    len(block0)    |        ...
|------------------|-------------------|-------------------|
|     2 bytes      |      3 bytes      |        ...
```

- Dictionary Checksum (32 bytes): Used for the compression dictionary checksum.
- Number of Batches (2 bytes uint16, little endian): Indicates the number of batches.
- For each batch:
  - Number of Blocks (2 bytes uint16, little endian): Indicates the number of blocks in the batch.
  - For each block in the batch: Length in bytes of the block (3 bytes uint24).

### Body

The body contains a list of "~RLP encoded" Lineth blocks. For each block, the raw data includes:

- Block Timestamp (uint64, little endian)
- List of RLP encoded Transactions (refer to EncodeTxForCompression for more details)

The raw data is compressed using [compress/lzss](https://github.com/consensys/compress), a snark-friendly compression algorithm. This allows the Lineth prover to "prove correct decompression of the blob".

### Final Blob: Byte Alignment

The header and the compressed body are concatenated. The resulting `[]byte` slice is packed into field elements. Due to hashing requirements and blob content verification, we can't fully utilize the 4096*32bytes; each 32byte is valid only if it is a valid field element.

The bls12377 scalar field bit size is 253; hence, we can only use up to 252bits out of the 256bits (in the 32bytes) in practice.

This packing operation may add some extra padding bytes to the blob.

See `blob.PackAlign` and `blob.UnpackAlign` for more info.

## Reference implementation

We provide `DecompressBlob(data, dictionary)` that decompresses the body and builds a `Blob` data structure in: TODO @gbotrel add link once open sourced.