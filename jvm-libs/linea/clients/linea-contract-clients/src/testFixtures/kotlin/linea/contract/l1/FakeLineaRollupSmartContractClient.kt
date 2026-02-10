package linea.contract.l1

import linea.domain.BlockParameter
import linea.kotlin.encodeHex
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.ConcurrentHashMap
import kotlin.random.Random
import kotlin.time.Clock
import kotlin.time.Instant

data class FinalizedBlock(
  val number: ULong,
  val timestamp: Instant,
  val stateRootHash: ByteArray,
) {
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as FinalizedBlock

    if (number != other.number) return false
    if (timestamp != other.timestamp) return false
    if (!stateRootHash.contentEquals(other.stateRootHash)) return false

    return true
  }

  override fun hashCode(): Int {
    var result = number.hashCode()
    result = 31 * result + timestamp.hashCode()
    result = 31 * result + stateRootHash.contentHashCode()
    return result
  }

  override fun toString(): String {
    return "FinalizedBlock(number=$number, timestamp=$timestamp, stateRootHash=${stateRootHash.encodeHex()})"
  }
}

class FakeLineaRollupSmartContractClient(
  val contractAddress: String = Random.nextBytes(20).encodeHex(),
  @get:Synchronized @set:Synchronized
  var contractVersion: LineaRollupContractVersion = LineaRollupContractVersion.V6,
  _finalizedBlocks: List<FinalizedBlock> = listOf(FinalizedBlock(0uL, Clock.System.now(), Random.nextBytes(32))),
  _messageRollingHashes: Map<ULong, ByteArray> = emptyMap(),
) : LineaRollupSmartContractClientReadOnly {
  val messageRollingHashes: MutableMap<ULong, ByteArray> = ConcurrentHashMap(_messageRollingHashes)
  val finalizedBlocks: MutableMap<ULong, FinalizedBlock> = ConcurrentHashMap()

  init {
    require(_finalizedBlocks.size > 0) { "At least one finalized block is required" }
    _finalizedBlocks.forEach { block -> finalizedBlocks.put(block.number, block) }
    require(finalizedBlocks[0UL] != null) {
      "Finalized block with number 0 must be present"
    }
  }

  private fun lastFinalizedBlock(): FinalizedBlock = finalizedBlocks.values.maxByOrNull { it.number }
    ?: throw IllegalStateException("No finalized blocks available")

  @Synchronized
  fun setFinalizedBlock(
    number: ULong,
    timestamp: Instant = Clock.System.now(),
    stateRootHash: ByteArray = Random.nextBytes(32),
  ) {
    val lastFinalizedBlock = lastFinalizedBlock().number

    require(lastFinalizedBlock <= number) {
      "next finalized blockNumber=$number must be greater than lastFinalizedBlock=$lastFinalizedBlock"
    }

    finalizedBlocks[number] = FinalizedBlock(number, timestamp, stateRootHash)
  }

  override fun getAddress(): String = contractAddress

  override fun getVersion(): SafeFuture<LineaRollupContractVersion> = SafeFuture.completedFuture(contractVersion)

  override fun finalizedL2BlockNumber(blockParameter: BlockParameter): SafeFuture<ULong> =
    SafeFuture.completedFuture(lastFinalizedBlock().number)

  override fun getMessageRollingHash(blockParameter: BlockParameter, messageNumber: Long): SafeFuture<ByteArray> =
    SafeFuture.completedFuture(messageRollingHashes[messageNumber.toULong()] ?: ByteArray(32))

  override fun isBlobShnarfPresent(blockParameter: BlockParameter, shnarf: ByteArray): SafeFuture<Boolean> =
    SafeFuture.completedFuture(false)

  override fun blockStateRootHash(blockParameter: BlockParameter, lineaL2BlockNumber: ULong): SafeFuture<ByteArray> {
    val stateRootHash = finalizedBlocks[lineaL2BlockNumber]?.stateRootHash ?: ByteArray(32)
    return SafeFuture.completedFuture(stateRootHash)
  }
}
