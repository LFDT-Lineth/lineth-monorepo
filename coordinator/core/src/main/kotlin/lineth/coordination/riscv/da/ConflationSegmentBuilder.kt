package lineth.coordination.riscv.da

import linea.domain.Block

// BLOB_BYTES_LENGTH = 4096 * 32 = 131_072 bytes (one EIP-4844 KZG blob).
// Defined here to mirror rollup_spec/src/rollup_spec/rollup.py BLOB_BYTES_LENGTH.
const val BLOB_BYTES_LENGTH: Int = 4096 * 32

// Length in bytes of the per-conflation segment length prefix in the DA stream.
// rollup_spec §3.1: stream layout is [len][lz4(rlp(conflation))] per conflation.
// 4 bytes in big-endian order (matching Python SEGMENT_LENGTH_PREFIX_BYTES = 4).
private const val SEGMENT_LENGTH_PREFIX_BYTES: Int = 4

/**
 * Builds the compressed byte segment for one conflation in the DA stream.
 *
 * DA stream layout (rollup_spec §3.1):
 *   [4-byte-len BE][lz4_block(rlp(truncatedBlocks))] || [4-byte-len BE][...] || ...
 *
 * Segments from multiple conflations are concatenated in order and sliced into
 * BLOB_BYTES_LENGTH-sized chunks. Each chunk becomes one KZG blob on L1.
 *
 * Implementation reference: rollup_spec/src/rollup_spec/rollup.py
 *   - truncate_block_rlp()           line ~734
 *   - rlp_encode_truncated_blocks()  line ~773
 *   - compress_lz4()                 line ~806
 *   - _compress_conflation_segment() line ~179
 */
object ConflationSegmentBuilder {

  /**
   * Builds the full compressed segment bytes for one conflation.
   *
   * Returns: [4-byte big-endian length][lz4-block-compressed RLP of truncated blocks]
   *
   * Steps (see Python reference for each):
   *   1. Truncate each block (strip tx signatures, recover ECDSA sender) — see [truncateBlock]
   *   2. RLP-encode the list of truncated blocks — see [rlpEncodeTruncatedBlocks]
   *   3. LZ4-block-compress the RLP bytes — see [compressLz4]
   *   4. Prepend a 4-byte big-endian length prefix of the compressed bytes
   */
  fun buildSegment(blocks: List<Block>, chainId: ULong): ByteArray {
    val truncated = blocks.map { truncateBlock(it, chainId) }
    val rlpEncoded = rlpEncodeTruncatedBlocks(truncated)
    val compressed = compressLz4(rlpEncoded)
    val prefix = compressed.size.to4BytesBigEndian()
    return prefix + compressed
  }

  /**
   * Returns the byte length of [buildSegment] without allocating the full result.
   *
   * Used by ConflatedChunkCoordinator to accumulate stream buffer size before sealing a chunk.
   */
  fun segmentSize(blocks: List<Block>, chainId: ULong): Int = buildSegment(blocks, chainId).size

  // ---------------------------------------------------------------------------
  // Internal steps — each maps directly to one function in the Python spec.
  // Replace each TODO with the real implementation.
  // ---------------------------------------------------------------------------

  /**
   * Produces a [TruncatedBlock] from a [Block] domain object by stripping transaction
   * signatures and recovering ECDSA sender addresses.
   *
   * Python reference: rollup_spec/src/rollup_spec/rollup.py truncate_block_rlp() ~line 734
   *
   * The [Block] domain object already carries all needed header fields — no RLP decoding
   * is required. Map fields directly:
   *   - timestamp    = block.timestamp
   *   - blockHash    = block.hash  (already keccak256(rlp_encode(header)), computed by the node)
   *   - prevRandao   = block.mixHash  (mixHash field holds prevRandao post-merge)
   *
   * For each [linea.domain.Transaction] in block.transactions:
   *   1. Produce signature-stripped canonical tx bytes (Python: _signature_stripped_tx_bytes()):
   *      Re-encode the tx WITHOUT the signature fields (r, s, v / yParity) in the canonical
   *      pre-signing wire format, per transaction type:
   *        FRONTIER:     RLP([nonce, gasPrice, gasLimit, to, value, data])
   *                      — or with EIP-155: RLP([nonce, gasPrice, gasLimit, to, value, data, chainId, 0, 0])
   *        ACCESS_LIST:  0x01 || RLP([chainId, nonce, gasPrice, gasLimit, to, value, data, accessList])
   *        EIP1559:      0x02 || RLP([chainId, nonce, maxPriorityFeePerGas, maxFeePerGas, gasLimit,
   *                                   to, value, data, accessList])
   *        DELEGATE_CODE:0x04 || RLP([chainId, nonce, maxPriorityFeePerGas, maxFeePerGas, gasLimit,
   *                                   to, value, data, accessList, authorizationList])
   *      All fields are available directly on [linea.domain.Transaction].
   *   2. Recover the sender address via ECDSA from (r, s, v/yParity) and the signing hash
   *      (keccak256 of the stripped tx bytes above):
   *        web3j: Sign.recoverFromSignature(recId, ECDSASignature(r, s), signingHash)
   *        → take bytes 12..31 of the recovered public key hash as the 20-byte address.
   */
  internal fun truncateBlock(block: Block, chainId: ULong): TruncatedBlock {
    TODO(
      "Implement block truncation from Block domain object: " +
        "map block.hash/timestamp/mixHash directly, then for each Transaction in block.transactions " +
        "produce signature-stripped bytes and recover ECDSA sender. " +
        "See rollup_spec/src/rollup_spec/rollup.py truncate_block_rlp() ~line 734.",
    )
  }

  /**
   * RLP-encodes a list of [TruncatedBlock]s into the canonical DA payload bytes.
   *
   * Python reference: rollup_spec/src/rollup_spec/rollup.py rlp_encode_truncated_blocks() ~line 773
   *
   * Both the sequencer (producing the blob) and the rollup guest (recomputing the
   * compressed payload for KZG verification) must use this exact encoding — any
   * deviation causes KZG verification to fail on L1.
   *
   * Wire layout (RLP list of lists):
   *   RLP([
   *     [uint(timestamp), bytes32(blockHash), bytes32(prevRandao),
   *      [strippedTx_1, strippedTx_2, ...],
   *      [from_1, from_2, ...]],
   *     ...
   *   ])
   *
   * The lineth.rlp.RLP helper already exists in the codebase (used by BlockRLPEncoder).
   */
  internal fun rlpEncodeTruncatedBlocks(blocks: List<TruncatedBlock>): ByteArray {
    TODO(
      "Implement RLP encoding of truncated blocks. Wire layout per " +
        "rollup_spec/src/rollup_spec/rollup.py rlp_encode_truncated_blocks() ~line 773. " +
        "Use lineth.rlp.RLP helpers already in the codebase.",
    )
  }

  /**
   * LZ4-block-compresses [data] using raw LZ4 block format (no uncompressed-size header).
   *
   * Python reference: rollup_spec/src/rollup_spec/rollup.py compress_lz4() ~line 806
   *   lz4.block.compress(data, store_size=False)
   *
   * The 'store_size=False' parameter means NO 4-byte uncompressed-size prepended —
   * only the raw LZ4 block bytes are returned. The length prefix in [buildSegment]
   * covers the compressed size; the decompressor must know the original size via
   * the RLP structure, not from an LZ4 header.
   *
   * Suggested JVM library: net.jpountz.lz4:lz4-java
   *   val factory = LZ4Factory.fastestInstance()
   *   val compressor = factory.fastCompressor()
   *   compressor.compress(data, 0, data.size, output, 0)
   *
   * The compression level must match the spec exactly — both the sequencer and the
   * RISC-V guest must produce byte-for-byte identical output for KZG to verify.
   * Verify against test vectors from rollup_spec before shipping.
   *
   * Dependency to add (coordinator/core/build.gradle or coordinator/app/build.gradle):
   *   implementation("org.lz4:lz4-java:<version>")
   */
  internal fun compressLz4(data: ByteArray): ByteArray {
    TODO(
      "Implement LZ4 block compression without size header (store_size=False equivalent). " +
        "Add net.jpountz.lz4:lz4-java dependency. Verify output matches Python " +
        "lz4.block.compress(data, store_size=False) byte-for-byte.",
    )
  }
}

/**
 * Per-conflation truncated block as defined in rollup_spec §3.2.
 *
 * This is the intermediate representation produced by [ConflationSegmentBuilder.truncateBlockRlp]
 * and consumed by [ConflationSegmentBuilder.rlpEncodeTruncatedBlocks].
 *
 * Fields mirror TruncatedEthereumBlock in rollup_spec/src/rollup_spec/rollup.py ~line 720:
 *   @dataclass
 *   class TruncatedEthereumBlock:
 *       timestamp: U64
 *       block_hash: Hash32
 *       prev_randao: Bytes32
 *       transactions: List[bytes]   # signature-stripped canonical tx bytes
 *       froms: List[Address]        # ECDSA-recovered sender per tx
 */
data class TruncatedBlock(
  val timestamp: ULong,
  val blockHash: ByteArray,   // 32 bytes, keccak256(rlp_encode(header))
  val prevRandao: ByteArray,  // 32 bytes
  val transactions: List<ByteArray>,  // signature-stripped tx bytes, one per tx
  val froms: List<ByteArray>,         // 20-byte sender address per tx
)

private fun Int.to4BytesBigEndian(): ByteArray = byteArrayOf(
  (this ushr 24).toByte(),
  (this ushr 16).toByte(),
  (this ushr 8).toByte(),
  this.toByte(),
)
