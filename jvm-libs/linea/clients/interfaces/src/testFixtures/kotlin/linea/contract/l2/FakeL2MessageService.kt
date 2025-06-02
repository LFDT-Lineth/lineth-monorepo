package linea.contract.l2

import linea.domain.BlockParameter
import linea.kotlin.encodeHex
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.random.Random

class FakeL2MessageService(
  val contractAddress: String = Random.nextBytes(20).encodeHex(),
  val contractDeployBlock: ULong = 0uL,
  var contractVersion: L2MessageServiceSmartContractVersion = L2MessageServiceSmartContractVersion.V1,
) : L2MessageServiceSmartContractClient {
  private val anchoredMessageHashes: MutableList<ByteArray> = mutableListOf()
  private val anchoredMessageRollingHashes: MutableMap<ULong, ByteArray> = mutableMapOf()

  private var lastAnchoredL1MessageNumber: ULong = 0uL
  private var lastAnchoredRollingHash: ByteArray = ByteArray(0)

  @get:Synchronized @set:Synchronized
  var forceAnchoringFailures: Boolean = false

  override fun getAddress(): String = contractAddress
  override fun getDeploymentBlock(): SafeFuture<ULong> = SafeFuture.completedFuture(contractDeployBlock)

  @Synchronized
  override fun getVersion(): SafeFuture<L2MessageServiceSmartContractVersion> =
    SafeFuture.completedFuture(contractVersion)

  @Synchronized
  fun setLastAnchoredL1Message(
    l1MessageNumber: ULong,
    rollingHash: ByteArray,
  ) {
    this.anchoredMessageRollingHashes[l1MessageNumber] = rollingHash
    this.lastAnchoredL1MessageNumber = l1MessageNumber
    this.lastAnchoredRollingHash = rollingHash
  }

  @Synchronized
  override fun anchorL1L2MessageHashes(
    messageHashes: List<ByteArray>,
    startingMessageNumber: ULong,
    finalMessageNumber: ULong,
    finalRollingHash: ByteArray,
  ): SafeFuture<String> {
    require(startingMessageNumber == lastAnchoredL1MessageNumber + 1UL) {
      "startingMessageNumber=$startingMessageNumber must be equal to " +
        "lastAnchoredL1MessageNumber=$lastAnchoredL1MessageNumber + 1"
    }
    require((finalMessageNumber - startingMessageNumber + 1UL).toInt() == messageHashes.size) {
      "finalMessageNumber=$finalMessageNumber - startingMessageNumber=$startingMessageNumber + 1UL " +
        "must be equal to messageHashes.size=${messageHashes.size}"
    }
    if (forceAnchoringFailures) {
      return SafeFuture.failedFuture(RuntimeException("FakeL2MessageService: forced anchoring failure"))
    }

    lastAnchoredL1MessageNumber = finalMessageNumber
    lastAnchoredRollingHash = finalRollingHash
    anchoredMessageHashes.addAll(messageHashes)
    anchoredMessageRollingHashes[finalMessageNumber] = finalRollingHash

    return SafeFuture.completedFuture(Random.nextBytes(32).encodeHex())
  }

  @Synchronized
  override fun getLastAnchoredL1MessageNumber(block: BlockParameter): SafeFuture<ULong> {
    return SafeFuture.completedFuture(lastAnchoredL1MessageNumber)
  }

  @Synchronized
  override fun getRollingHashByL1MessageNumber(block: BlockParameter, l1MessageNumber: ULong): SafeFuture<ByteArray> {
    return SafeFuture.completedFuture(anchoredMessageRollingHashes.getOrDefault(l1MessageNumber, ByteArray(32)))
  }

  @Synchronized
  fun getLastAnchoredRollingHash(): ByteArray {
    return lastAnchoredRollingHash
  }

  @Synchronized
  fun getAnchoredMessageHashes(): List<ByteArray> = anchoredMessageHashes.toList()

  @Synchronized
  fun getAnchoredMessagesRollingHashes(): Map<ULong, ByteArray> = anchoredMessageRollingHashes.toMap()
}
