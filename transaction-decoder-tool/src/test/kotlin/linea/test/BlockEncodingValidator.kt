package linea.test

import io.vertx.core.Vertx
import linea.blob.BlobCompressorVersion
import linea.blob.GoBackedBlobCompressor
import linea.domain.Block
import linea.domain.CommonDomainFunctions
import linea.domain.toBesu
import linea.rlp.BesuRlpDecoderAsyncVertxImpl
import linea.rlp.BesuRlpMainnetEncoderAsyncVertxImpl
import linea.rlp.RLP
import net.consensys.linea.blob.BlobDecompressorVersion
import net.consensys.linea.blob.GoNativeBlobDecompressorFactory
import net.consensys.zkevm.PeriodicPollingService
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.fail
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.ConcurrentLinkedQueue
import java.util.concurrent.atomic.AtomicReference
import kotlin.time.Duration.Companion.milliseconds

// 100MB, much larger than a real blob, but just for testing to allow faster testing by compressing more blocks
val BLOB_COMPRESSOR_SIZE: UInt = 100u * 1024u * 1024U

class BlockEncodingValidator(
  val vertx: Vertx,
  val compressorVersion: BlobCompressorVersion = BlobCompressorVersion.V1_2,
  val decompressorVersion: BlobDecompressorVersion = BlobDecompressorVersion.V1_2_0,
  val blobSizeLimitBytes: UInt = BLOB_COMPRESSOR_SIZE,
  val log: Logger = LogManager.getLogger(BlockEncodingValidator::class.java),
) : PeriodicPollingService(vertx, pollingIntervalMs = 1.milliseconds.inWholeMilliseconds, log = log) {

  val compressor = GoBackedBlobCompressor.getInstance(compressorVersion, blobSizeLimitBytes.toInt())
  val decompressor = GoNativeBlobDecompressorFactory.getInstance(decompressorVersion)
  val rlpEncoder = BesuRlpMainnetEncoderAsyncVertxImpl(vertx)
  val rlpMainnetDecoder = BesuRlpDecoderAsyncVertxImpl.mainnetDecoder(vertx)
  val rlpBlobDecoder = BesuRlpDecoderAsyncVertxImpl.blobDecoder(vertx)
  val queueOfBlocksToValidate = ConcurrentLinkedQueue<Block>()
  var highestValidatedBlockNumber = AtomicReference<ULong>(ULong.MIN_VALUE)

  override fun action(): SafeFuture<*> {
    return validateCycle()
  }

  fun validateRlpEncodingDecoding(blocks: List<Block>): SafeFuture<Unit> {
    val besuBlocks = blocks.map { it.toBesu() }
    return rlpEncoder.encodeAsync(besuBlocks)
      .thenCompose { encodedBlocks ->
        rlpMainnetDecoder.decodeAsync(encodedBlocks)
      }
      .thenApply { decodedBlocks ->
        val unMatchingBlocks = besuBlocks.zip(decodedBlocks).filter { (expected, actual) -> expected != actual }
        if (unMatchingBlocks.isEmpty()) {
          log.info(
            "all blocks encoding/decoding match: blocks={}",
            CommonDomainFunctions.blockIntervalString(blocks.first().number, blocks.last().number),
          )
        } else {
          unMatchingBlocks.forEach { (expected, actual) ->
            log.error(
              "block encoding/decoding mismatch: block={} \nexpected={} \nactual={}",
              expected.header.number,
              expected,
              actual,
            )
          }
        }
      }
  }

  fun validateCompression(blocks: List<Block>): SafeFuture<Unit> {
    queueOfBlocksToValidate.addAll(blocks)
    return SafeFuture.completedFuture(Unit)
  }

  fun validateCycle(): SafeFuture<Unit> {
    val blocks = queueOfBlocksToValidate.pull(300)
    if (blocks.isEmpty()) {
      return SafeFuture.completedFuture(Unit)
    }
    log.info(
      "compression validation blocks={} started",
      CommonDomainFunctions.blockIntervalString(blocks.first().number, blocks.last().number),
    )
    val besuBlocks = blocks.map { it.toBesu() }
    val originalBlockInterval = CommonDomainFunctions.blockIntervalString(blocks.first().number, blocks.last().number)
    return rlpEncoder.encodeAsync(besuBlocks)
      .thenCompose { encodedBlocks ->
        encodedBlocks.forEach { compressor.appendBlock(it) }
        val compressedData = compressor.getCompressedData()
        compressor.reset()
        val decompressedData = decompressor.decompress(compressedData)
        val decompressedBlocksList = RLP.decodeList(decompressedData)
        rlpBlobDecoder.decodeAsync(decompressedBlocksList)
      }.thenApply { decompressedBlocks ->
        assertThat(decompressedBlocks.size).isEqualTo(besuBlocks.size)
          .withFailMessage(
            // this can happen if not all blocks fit into compressor limit
            "originalBlocks=$originalBlockInterval decompressedBlocks.size=${decompressedBlocks.size} != ",
          )
        decompressedBlocks.zip(besuBlocks).forEach { (decompressed, original) ->
          runCatching {
            assertBlock(decompressed, original)
          }.getOrElse {
            log.error(
              "Decompressed block={} does not match: error={}",
              original.header.number,
              it.message,
              it,
            )
          }
        }
      }
      .thenPeek {
        highestValidatedBlockNumber.set(highestValidatedBlockNumber.get().coerceAtLeast(blocks.last().number))
        log.info(
          "compression validation blocks={} finished",
          originalBlockInterval,
        )
      }
  }
}

fun <T> ConcurrentLinkedQueue<T>.pull(elementsLimit: Int): List<T> {
  val elements = mutableListOf<T>()
  var element = poll()
  while (element != null && elements.size < elementsLimit) {
    elements.add(element)
    element = poll()
  }
  return elements
}

fun assertBlock(
  decompressedBlock: org.hyperledger.besu.ethereum.core.Block,
  originalBlock: org.hyperledger.besu.ethereum.core.Block,
  log: Logger = LogManager.getLogger("test.assert.Block"),
) {
  // on decompression, the hash is placed as parentHash because besu recomputes the hash
  // but custom decoder overrides hash calculation to use parentHash
  assertThat(decompressedBlock.header.timestamp).isEqualTo(originalBlock.header.timestamp)
  assertThat(decompressedBlock.header.hash).isEqualTo(originalBlock.header.hash)

  decompressedBlock.body.transactions.forEachIndexed { index, decompressedTx ->
    val originalTx = originalBlock.body.transactions[index]
    log.trace(
      "block={} txIndex={} \n originalTx={} \n decodedTx={} \n originalTxRlp={}",
      originalBlock.header.number,
      index,
      originalTx,
      decompressedTx,

      originalTx.encoded(),
    )
    runCatching {
      assertThat(decompressedTx.type).isEqualTo(originalTx.type)
      assertThat(decompressedTx.sender).isEqualTo(originalTx.sender)
      assertThat(decompressedTx.nonce).isEqualTo(originalTx.nonce)
      assertThat(decompressedTx.gasLimit).isEqualTo(originalTx.gasLimit)
      if (originalTx.type.supports1559FeeMarket()) {
        assertThat(decompressedTx.maxFeePerGas).isEqualTo(originalTx.maxFeePerGas)
        assertThat(decompressedTx.maxPriorityFeePerGas).isEqualTo(originalTx.maxPriorityFeePerGas)
      } else {
        assertThat(decompressedTx.gasPrice).isEqualTo(originalTx.gasPrice)
      }
      assertThat(decompressedTx.to).isEqualTo(originalTx.to)
      assertThat(decompressedTx.value).isEqualTo(originalTx.value)
      assertThat(decompressedTx.accessList).isEqualTo(originalTx.accessList)
      assertThat(decompressedTx.payload).isEqualTo(originalTx.payload)
    }.getOrElse { th ->
      fail(
        "Transaction does not match: block=${originalBlock.header.number} " +
          "txIndex=$index error=${th.message} origTxRlp=${originalTx.encoded()}",
        th,
      )
    }
  }
}
