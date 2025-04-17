package linea.staterecovery

import io.vertx.core.Vertx
import kotlinx.datetime.Clock
import kotlinx.datetime.Instant
import linea.domain.BinaryDecoder
import linea.domain.CommonDomainFunctions
import linea.kotlin.decodeHex
import linea.kotlin.encodeHex
import linea.rlp.BesuRlpBlobDecoder
import linea.rlp.RLP
import net.consensys.linea.async.toSafeFuture
import net.consensys.linea.blob.BlobDecompressor
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.hyperledger.besu.ethereum.core.Block
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.Callable
import kotlin.jvm.optionals.getOrNull

interface BlobDecompressorAndDeserializer {
  /**
   * Decompresses the EIP4844 blobs and deserializes them into domain objects.
   */
  fun decompress(
    startBlockNumber: ULong,
    blobs: List<ByteArray>
  ): SafeFuture<List<BlockFromL1RecoveredData>>
}

data class BlockHeaderStaticFields(
  val coinbase: ByteArray,
  val gasLimit: ULong = 2_000_000_000UL,
  val difficulty: ULong = 2UL
) {
  companion object {
    val localDev = BlockHeaderStaticFields(
      coinbase = "0x6d976c9b8ceee705d4fe8699b44e5eb58242f484".decodeHex()
    )
  }

  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as BlockHeaderStaticFields

    if (!coinbase.contentEquals(other.coinbase)) return false
    if (gasLimit != other.gasLimit) return false
    if (difficulty != other.difficulty) return false

    return true
  }

  override fun hashCode(): Int {
    var result = coinbase.contentHashCode()
    result = 31 * result + gasLimit.hashCode()
    result = 31 * result + difficulty.hashCode()
    return result
  }

  override fun toString(): String {
    return "BlockHeaderStaticFields(coinbase=${coinbase.encodeHex()}, gasLimit=$gasLimit, difficulty=$difficulty)"
  }
}

class BlobDecompressorToDomainV1(
  val decompressor: BlobDecompressor,
  val staticFields: BlockHeaderStaticFields,
  val vertx: Vertx,
  val decoder: BinaryDecoder<Block> = BesuRlpBlobDecoder,
  val logger: Logger = LogManager.getLogger(BlobDecompressorToDomainV1::class.java)
) : BlobDecompressorAndDeserializer {
  override fun decompress(
    startBlockNumber: ULong,
    blobs: List<ByteArray>
  ): SafeFuture<List<BlockFromL1RecoveredData>> {
    var blockNumber = startBlockNumber
    val startTime = Clock.System.now()
    logger.trace("start decompressing blobs: startBlockNumber={} {} blobs", startBlockNumber, blobs.size)
    val decompressedBlobs = blobs.map { decompressor.decompress(it) }
    return SafeFuture
      .collectAll(decompressedBlobs.map(::decodeBlocksAsync).stream())
      .thenApply { blobsBlocks: List<List<Block>> ->
        blobsBlocks.flatten().map { block ->
          val header = BlockHeaderFromL1RecoveredData(
            blockNumber = blockNumber++,
            blockHash = block.header.hash.toArray(),
            coinbase = staticFields.coinbase,
            blockTimestamp = Instant.fromEpochSeconds(block.header.timestamp),
            gasLimit = this.staticFields.gasLimit,
            difficulty = this.staticFields.difficulty
          )
          val transactions = block.body.transactions.map { transaction ->
            TransactionFromL1RecoveredData(
              type = transaction.type.serializedType.toUByte(),
              from = transaction.sender.toArray(),
              nonce = transaction.nonce.toULong(),
              gasLimit = transaction.gasLimit.toULong(),
              maxFeePerGas = transaction.maxFeePerGas.getOrNull()?.asBigInteger,
              maxPriorityFeePerGas = transaction.maxPriorityFeePerGas.getOrNull()?.asBigInteger,
              gasPrice = transaction.gasPrice.getOrNull()?.asBigInteger,
              to = transaction.to.getOrNull()?.toArray(),
              value = transaction.value.asBigInteger,
              data = transaction.payload.toArray(),
              accessList = transaction.accessList.getOrNull()?.map { accessTuple ->
                TransactionFromL1RecoveredData.AccessTuple(
                  address = accessTuple.address.toArray(),
                  storageKeys = accessTuple.storageKeys.map { it.toArray() }
                )
              }
            )
          }
          BlockFromL1RecoveredData(
            header = header,
            transactions = transactions
          )
        }
      }.thenPeek {
        val endTime = Clock.System.now()
        logger.debug(
          "blobs decompressed and serialized: duration={} blobsCount={} blocks={}",
          endTime - startTime,
          blobs.size,
          CommonDomainFunctions.blockIntervalString(startBlockNumber, blockNumber - 1UL)
        )
      }
  }

  private fun decodeBlocksAsync(blocksRLP: ByteArray): SafeFuture<List<Block>> {
    return vertx.executeBlocking(
      Callable { RLP.decodeList(blocksRLP).map(decoder::decode) },
      false
    )
      .onFailure(logger::error)
      .toSafeFuture()
  }
}
