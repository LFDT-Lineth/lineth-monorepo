package linea.staterecovery

import io.vertx.core.Vertx
import linea.blob.BlobCompressor
import linea.blob.BlobCompressorVersion
import linea.blob.GoBackedBlobCompressor
import linea.kotlin.encodeHex
import linea.rlp.RLP
import net.consensys.linea.blob.BlobDecompressorVersion
import net.consensys.linea.blob.GoNativeBlobDecompressorFactory
import net.consensys.linea.nativecompressor.CompressorTestData
import org.apache.tuweni.bytes.Bytes32
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.fail
import org.hyperledger.besu.datatypes.Address
import org.hyperledger.besu.ethereum.core.Block
import org.hyperledger.besu.ethereum.core.Transaction
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.TestInstance
import org.opentest4j.AssertionFailedError
import kotlin.jvm.optionals.getOrNull
import kotlin.time.Instant

@TestInstance(TestInstance.Lifecycle.PER_CLASS)
class BlobDecompressorAndDeserializerV1Test {
  private lateinit var compressor: BlobCompressor
  private val blockStaticFields = BlockHeaderStaticFields(
    coinbase = Address.ZERO.toArray(),
    gasLimit = 30_000_000UL,
    difficulty = 0UL,
  )
  private lateinit var decompressorToDomain: BlobDecompressorAndDeserializer
  private lateinit var vertx: Vertx

  @BeforeEach
  fun setUp() {
    vertx = Vertx.vertx()
    compressor = GoBackedBlobCompressor.getInstance(
      compressorVersion = BlobCompressorVersion.V1_2,
      dataLimit = 124 * 1024,
    )
    val decompressor = GoNativeBlobDecompressorFactory.getInstance(BlobDecompressorVersion.V1_2_0)
    decompressorToDomain = BlobDecompressorToDomainV1(decompressor, blockStaticFields, vertx)
  }

  @AfterEach
  fun afterEach() {
    vertx.close()
  }

  @Test
  fun `should decompress block and transactions`() {
    val blocksRLP = CompressorTestData.blocksRlpEncoded
    assertBlockCompressionAndDecompression(blocksRLP)
  }

  private fun assertBlockCompressionAndDecompression(blocksRLP: List<ByteArray>) {
    val blocks = blocksRLP.map(RLP::decodeBlockWithMainnetFunctions)
    val startingBlockNumber = blocks[0].header.number.toULong()

    val blobs = blocks.chunked(2).map { compressBlocks(it) }

    val recoveredBlocks = decompressorToDomain.decompress(
      startBlockNumber = startingBlockNumber,
      blobs = blobs,
    ).get()
    assertThat(recoveredBlocks[0].header.blockNumber).isEqualTo(startingBlockNumber)

    recoveredBlocks.zip(blocks) { recoveredBlock, originalBlock ->
      assertBlockData(recoveredBlock, originalBlock)
    }
  }

  private fun assertBlockData(uncompressed: BlockFromL1RecoveredData, original: Block) {
    try {
      assertThat(uncompressed.header.blockNumber).isEqualTo(original.header.number.toULong())
      assertThat(uncompressed.header.blockHash.encodeHex()).isEqualTo(original.header.hash.toArray().encodeHex())
      assertThat(uncompressed.header.coinbase).isEqualTo(blockStaticFields.coinbase)
      assertThat(uncompressed.header.blockTimestamp).isEqualTo(Instant.fromEpochSeconds(original.header.timestamp))
      assertThat(uncompressed.header.gasLimit).isEqualTo(blockStaticFields.gasLimit)
      assertThat(uncompressed.header.difficulty).isEqualTo(blockStaticFields.difficulty)
      uncompressed.transactions.zip(original.body.transactions) { uncompressedTransaction, originalTransaction ->
        assertTransactionData(uncompressedTransaction, originalTransaction)
      }
    } catch (e: AssertionFailedError) {
      fail(
        "uncompressed block does not match expected original: blockNumber: ${e.message} " +
          "\n original    =$original " +
          "\n uncompressed=$uncompressed ",
        e,
      )
    }
  }

  private fun assertTransactionData(uncompressed: TransactionFromL1RecoveredData, original: Transaction) {
    assertThat(uncompressed.type).isEqualTo(original.type.serializedType.toUByte())
    assertThat(uncompressed.from).isEqualTo(original.sender.toArray())
    assertThat(uncompressed.nonce).isEqualTo(original.nonce.toULong())
    assertThat(uncompressed.to).isEqualTo(original.to.getOrNull()?.toArray())
    assertThat(uncompressed.gasLimit).isEqualTo(original.gasLimit.toULong())
    assertThat(uncompressed.maxFeePerGas).isEqualTo(original.maxFeePerGas.getOrNull()?.asBigInteger)
    assertThat(uncompressed.maxPriorityFeePerGas).isEqualTo(original.maxPriorityFeePerGas.getOrNull()?.asBigInteger)
    assertThat(uncompressed.gasPrice).isEqualTo(original.gasPrice.getOrNull()?.asBigInteger)
    assertThat(uncompressed.value).isEqualTo(original.value.asBigInteger)
    assertThat(uncompressed.data?.encodeHex()).isEqualTo(original.payload?.toArray()?.encodeHex())
    if (uncompressed.accessList.isNullOrEmpty() != original.accessList.getOrNull().isNullOrEmpty()) {
      assertThat(uncompressed.accessList).isEqualTo(original.accessList.getOrNull())
    } else {
      uncompressed.accessList?.zip(original.accessList.getOrNull()!!) { a, b ->
        assertThat(a.address).isEqualTo(b.address.toArray())
        assertThat(a.storageKeys.map { Bytes32.wrap(it) }).isEqualTo(b.storageKeys)
      }
    }
  }

  private fun compressBlocks(blocks: List<Block>): ByteArray {
    return compress(blocks.map { block -> block.toRlp().toArray() })
  }

  private fun compress(blocks: List<ByteArray>): ByteArray {
    blocks.forEach { blockRlp ->
      compressor.appendBlock(blockRlp)
    }

    return compressor.getCompressedDataAndReset()
  }
}
