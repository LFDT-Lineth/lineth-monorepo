package linea.contract.l1

import linea.EthLogsSearcher
import linea.SearchDirection
import linea.contract.FAKE_READ_ONLY_CREDENTIALS
<<<<<<< HEAD
<<<<<<< HEAD:jvm-libs/linea/clients/linea-contract-clients/src/main/kotlin/linea/contract/l1/Web3JLinethRollupSmartContractClientReadOnly.kt
import linea.contract.LinethRollupV6
import linea.contract.LinethRollupV8
=======
import linea.contract.LineaRollupV6
import linea.contract.LineaRollupV8
<<<<<<< HEAD:jvm-libs/linea/clients/linea-contract-clients/src/main/kotlin/linea/contract/l1/Web3JLinethRollupSmartContractClientReadOnly.kt
>>>>>>> 5c0bae8ac (chore(coordinator): adds initial scafold for contract v9 RISC-V (#3618)):jvm-libs/linea/clients/linea-contract-clients/src/main/kotlin/linea/contract/l1/Web3JLineaRollupSmartContractClientReadOnly.kt
=======
>>>>>>> 5c0bae8ac (chore(coordinator): adds initial scafold for contract v9 RISC-V (#3618)):jvm-libs/linea/clients/linea-contract-clients/src/main/kotlin/linea/contract/l1/Web3JLineaRollupSmartContractClientReadOnly.kt
=======
import linea.contract.LinethRollupV6
import linea.contract.LinethRollupV8
>>>>>>> 53054e4e0 (chore(coordinator): rename LineaRollup to LinethRollup in JVM components (#3421))
import linea.contract.LinethRollupV9
import linea.contract.events.FinalizedStateUpdatedEvent
import linea.domain.BlockParameter
import linea.domain.EthLogEvent
import linea.domain.toBlockParameter
import linea.kotlin.toBigInteger
import linea.kotlin.toULong
import linea.web3j.domain.toWeb3j
import net.consensys.linea.async.toSafeFuture
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.web3j.protocol.Web3j
import org.web3j.tx.Contract
import org.web3j.tx.gas.StaticGasProvider
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.math.BigInteger
import java.util.concurrent.atomic.AtomicReference
import kotlin.time.Clock
import kotlin.time.Duration
import kotlin.time.Duration.Companion.seconds
import kotlin.time.Instant

open class Web3JLinethRollupSmartContractClientReadOnly(
  val web3j: Web3j,
  val contractAddress: String,
  private val ethLogsSearcher: EthLogsSearcher,
  private val l1EventSearchMaxBlockRange: UInt = 10_000u,
  private val finalizedStateSearchInitialBlockParameter: BlockParameter = BlockParameter.Tag.EARLIEST,
  private val versionRefreshInterval: Duration = 6.seconds,
  private val clock: Clock = Clock.System,
<<<<<<< HEAD
<<<<<<< HEAD:jvm-libs/linea/clients/linea-contract-clients/src/main/kotlin/linea/contract/l1/Web3JLinethRollupSmartContractClientReadOnly.kt
<<<<<<< HEAD:jvm-libs/linea/clients/linea-contract-clients/src/main/kotlin/linea/contract/l1/Web3JLinethRollupSmartContractClientReadOnly.kt
  private val log: Logger = LogManager.getLogger(Web3JLinethRollupSmartContractClientReadOnly::class.java),
) : LinethRollupSmartContractClientReadOnly,
  LinethRollupSmartContractClientReadOnlyFinalizedStateProvider,
=======
=======
>>>>>>> 5c0bae8ac (chore(coordinator): adds initial scafold for contract v9 RISC-V (#3618)):jvm-libs/linea/clients/linea-contract-clients/src/main/kotlin/linea/contract/l1/Web3JLineaRollupSmartContractClientReadOnly.kt
  private val log: Logger = LogManager.getLogger(Web3JLineaRollupSmartContractClientReadOnly::class.java),
) : LineaRollupSmartContractClientReadOnly,
  LineaRollupSmartContractClientReadOnlyFinalizedStateProvider,
>>>>>>> 5c0bae8ac (chore(coordinator): adds initial scafold for contract v9 RISC-V (#3618)):jvm-libs/linea/clients/linea-contract-clients/src/main/kotlin/linea/contract/l1/Web3JLineaRollupSmartContractClientReadOnly.kt
=======
  private val log: Logger = LogManager.getLogger(Web3JLinethRollupSmartContractClientReadOnly::class.java),
) : LinethRollupSmartContractClientReadOnly,
  LinethRollupSmartContractClientReadOnlyFinalizedStateProvider,
>>>>>>> 53054e4e0 (chore(coordinator): rename LineaRollup to LinethRollup in JVM components (#3421))
  FinalizedStateDataProvider {

  protected fun contractClientV8AtBlock(blockParameter: BlockParameter): LinethRollupV8 {
    return contractClientAtBlock(blockParameter, LinethRollupV8::class.java)
<<<<<<< HEAD
  }

  protected fun contractClientV9AtBlock(blockParameter: BlockParameter): LinethRollupV9 {
    return contractClientAtBlock(blockParameter, LinethRollupV9::class.java)
  }

  protected fun contractClientV9AtBlock(blockParameter: BlockParameter): LinethRollupV9 {
    return contractClientAtBlock(blockParameter, LinethRollupV9::class.java)
=======
>>>>>>> 53054e4e0 (chore(coordinator): rename LineaRollup to LinethRollup in JVM components (#3421))
  }

  protected fun contractClientV9AtBlock(blockParameter: BlockParameter): LinethRollupV9 {
    return contractClientAtBlock(blockParameter, LinethRollupV9::class.java)
  }

  protected fun <T : Contract> loadContractClient(contract: Class<T>): T {
    @Suppress("UNCHECKED_CAST")
    return when {
      LinethRollupV6::class.java.isAssignableFrom(contract) -> LinethRollupV6.load(
        contractAddress,
        web3j,
        FAKE_READ_ONLY_CREDENTIALS,
        StaticGasProvider(BigInteger.ZERO, BigInteger.ZERO),
      )

      LinethRollupV8::class.java.isAssignableFrom(contract) -> LinethRollupV8.load(
        contractAddress,
        web3j,
        FAKE_READ_ONLY_CREDENTIALS,
        StaticGasProvider(BigInteger.ZERO, BigInteger.ZERO),
      )

      LinethRollupV9::class.java.isAssignableFrom(contract) -> LinethRollupV9.load(
        contractAddress,
        web3j,
        FAKE_READ_ONLY_CREDENTIALS,
        StaticGasProvider(BigInteger.ZERO, BigInteger.ZERO),
      )

      else -> throw IllegalArgumentException("Unsupported contract type: ${contract.name}")
    } as T
  }

  protected fun <T : Contract> contractClientAtBlock(blockParameter: BlockParameter, contract: Class<T>): T {
    @Suppress("UNCHECKED_CAST")
    return loadContractClient(contract).apply {
      this.setDefaultBlockParameter(blockParameter.toWeb3j())
    }
  }

  private data class CachedVersion(
<<<<<<< HEAD
<<<<<<< HEAD:jvm-libs/linea/clients/linea-contract-clients/src/main/kotlin/linea/contract/l1/Web3JLinethRollupSmartContractClientReadOnly.kt
<<<<<<< HEAD:jvm-libs/linea/clients/linea-contract-clients/src/main/kotlin/linea/contract/l1/Web3JLinethRollupSmartContractClientReadOnly.kt
    val version: LinethRollupContractVersion,
    val fetchedAt: Instant,
  )

  private val smartContractVersionCache = AtomicReference<CachedVersion>(null)

  private fun getSmartContractVersion(): SafeFuture<LinethRollupContractVersion> {
    val cached = smartContractVersionCache.get()
    return when {
      // once upgraded, it's not downgraded
      cached?.version == LinethRollupContractVersion.latest ->
        SafeFuture.completedFuture(LinethRollupContractVersion.latest)
=======
=======
>>>>>>> 5c0bae8ac (chore(coordinator): adds initial scafold for contract v9 RISC-V (#3618)):jvm-libs/linea/clients/linea-contract-clients/src/main/kotlin/linea/contract/l1/Web3JLineaRollupSmartContractClientReadOnly.kt
    val version: LineaRollupContractVersion,
=======
    val version: LinethRollupContractVersion,
>>>>>>> 53054e4e0 (chore(coordinator): rename LineaRollup to LinethRollup in JVM components (#3421))
    val fetchedAt: Instant,
  )

  private val smartContractVersionCache = AtomicReference<CachedVersion>(null)

  private fun getSmartContractVersion(): SafeFuture<LinethRollupContractVersion> {
    val cached = smartContractVersionCache.get()
    return when {
      // once upgraded, it's not downgraded
<<<<<<< HEAD
      cached?.version == LineaRollupContractVersion.latest ->
        SafeFuture.completedFuture(LineaRollupContractVersion.latest)
<<<<<<< HEAD:jvm-libs/linea/clients/linea-contract-clients/src/main/kotlin/linea/contract/l1/Web3JLinethRollupSmartContractClientReadOnly.kt
>>>>>>> 5c0bae8ac (chore(coordinator): adds initial scafold for contract v9 RISC-V (#3618)):jvm-libs/linea/clients/linea-contract-clients/src/main/kotlin/linea/contract/l1/Web3JLineaRollupSmartContractClientReadOnly.kt
=======
>>>>>>> 5c0bae8ac (chore(coordinator): adds initial scafold for contract v9 RISC-V (#3618)):jvm-libs/linea/clients/linea-contract-clients/src/main/kotlin/linea/contract/l1/Web3JLineaRollupSmartContractClientReadOnly.kt
=======
      cached?.version == LinethRollupContractVersion.latest ->
        SafeFuture.completedFuture(LinethRollupContractVersion.latest)
>>>>>>> 53054e4e0 (chore(coordinator): rename LineaRollup to LinethRollup in JVM components (#3421))

      // below latest: serve from cache within the refresh interval so repeated getVersion calls
      // don't refetch on every call, while still detecting an upgrade within versionRefreshInterval
      cached != null && clock.now() < cached.fetchedAt + versionRefreshInterval ->
        SafeFuture.completedFuture(cached.version)

      else ->
        fetchSmartContractVersion()
          .thenPeek { fetchedVersion ->
            val current = smartContractVersionCache.get()
            // prevent inflight request to override and rollback
            if (current == null || fetchedVersion >= current.version) {
              smartContractVersionCache.set(CachedVersion(fetchedVersion, clock.now()))

              if (current != null && fetchedVersion != current.version) {
                log.info(
                  "Smart contract upgraded: prevVersion={} upgradedVersion={}",
                  current.version,
                  fetchedVersion,
                )
              }
            }
          }
    }
  }

  internal open fun fetchSmartContractVersion(
    blockParameter: BlockParameter = BlockParameter.Tag.LATEST,
  ): SafeFuture<LinethRollupContractVersion> {
    return contractClientAtBlock(blockParameter, LinethRollupV6::class.java)
      .CONTRACT_VERSION()
      .sendAsync()
      .toSafeFuture()
      .thenApply(::parseContractVersion)
  }

<<<<<<< HEAD
<<<<<<< HEAD:jvm-libs/linea/clients/linea-contract-clients/src/main/kotlin/linea/contract/l1/Web3JLinethRollupSmartContractClientReadOnly.kt
<<<<<<< HEAD:jvm-libs/linea/clients/linea-contract-clients/src/main/kotlin/linea/contract/l1/Web3JLinethRollupSmartContractClientReadOnly.kt
  internal fun parseContractVersion(version: String): LinethRollupContractVersion {
    return when {
      version.startsWith("6") -> LinethRollupContractVersion.V6
      version.startsWith("7") -> LinethRollupContractVersion.V7
      version.startsWith("8") -> LinethRollupContractVersion.V8
      version.startsWith("9") -> LinethRollupContractVersion.V9
=======
=======
>>>>>>> 5c0bae8ac (chore(coordinator): adds initial scafold for contract v9 RISC-V (#3618)):jvm-libs/linea/clients/linea-contract-clients/src/main/kotlin/linea/contract/l1/Web3JLineaRollupSmartContractClientReadOnly.kt
  internal fun parseContractVersion(version: String): LineaRollupContractVersion {
    return when {
      version.startsWith("6") -> LineaRollupContractVersion.V6
      version.startsWith("7") -> LineaRollupContractVersion.V7
      version.startsWith("8") -> LineaRollupContractVersion.V8
      version.startsWith("9") -> LineaRollupContractVersion.V9
<<<<<<< HEAD:jvm-libs/linea/clients/linea-contract-clients/src/main/kotlin/linea/contract/l1/Web3JLinethRollupSmartContractClientReadOnly.kt
>>>>>>> 5c0bae8ac (chore(coordinator): adds initial scafold for contract v9 RISC-V (#3618)):jvm-libs/linea/clients/linea-contract-clients/src/main/kotlin/linea/contract/l1/Web3JLineaRollupSmartContractClientReadOnly.kt
=======
>>>>>>> 5c0bae8ac (chore(coordinator): adds initial scafold for contract v9 RISC-V (#3618)):jvm-libs/linea/clients/linea-contract-clients/src/main/kotlin/linea/contract/l1/Web3JLineaRollupSmartContractClientReadOnly.kt
=======
  internal fun parseContractVersion(version: String): LinethRollupContractVersion {
    return when {
      version.startsWith("6") -> LinethRollupContractVersion.V6
      version.startsWith("7") -> LinethRollupContractVersion.V7
      version.startsWith("8") -> LinethRollupContractVersion.V8
      version.startsWith("9") -> LinethRollupContractVersion.V9
>>>>>>> 53054e4e0 (chore(coordinator): rename LineaRollup to LinethRollup in JVM components (#3421))
      else -> throw IllegalStateException("Unsupported contract version: $version")
    }
  }

  override fun getAddress(): String = contractAddress

  override fun getVersion(blockParameter: BlockParameter): SafeFuture<LinethRollupContractVersion> {
    return if (blockParameter == BlockParameter.Tag.LATEST) {
      getSmartContractVersion()
    } else {
      fetchSmartContractVersion(blockParameter)
    }
  }

  override fun finalizedL2BlockNumber(blockParameter: BlockParameter): SafeFuture<ULong> {
    return contractClientV8AtBlock(blockParameter)
      .currentL2BlockNumber().sendAsync()
      .thenApply { it.toULong() }
      .toSafeFuture()
  }

  override fun getMessageRollingHash(blockParameter: BlockParameter, messageNumber: Long): SafeFuture<ByteArray> {
    require(messageNumber >= 0) { "messageNumber must be greater than or equal to 0" }

    return contractClientV8AtBlock(
      blockParameter,
    ).rollingHashes(messageNumber.toBigInteger()).sendAsync().toSafeFuture()
  }

  override fun isBlobShnarfPresent(blockParameter: BlockParameter, shnarf: ByteArray): SafeFuture<Boolean> {
    return getVersion()
      .thenCompose { version ->
        when (version) {
          LinethRollupContractVersion.V6,
          LinethRollupContractVersion.V7,
          LinethRollupContractVersion.V8,
          -> contractClientV8AtBlock(blockParameter).blobShnarfExists(shnarf)
<<<<<<< HEAD
<<<<<<< HEAD:jvm-libs/linea/clients/linea-contract-clients/src/main/kotlin/linea/contract/l1/Web3JLinethRollupSmartContractClientReadOnly.kt
<<<<<<< HEAD:jvm-libs/linea/clients/linea-contract-clients/src/main/kotlin/linea/contract/l1/Web3JLinethRollupSmartContractClientReadOnly.kt
          LinethRollupContractVersion.V9 -> // just ensure not regression while WIP
=======
          LineaRollupContractVersion.V9 -> // just ensure not regression while WIP
>>>>>>> 5c0bae8ac (chore(coordinator): adds initial scafold for contract v9 RISC-V (#3618)):jvm-libs/linea/clients/linea-contract-clients/src/main/kotlin/linea/contract/l1/Web3JLineaRollupSmartContractClientReadOnly.kt
=======
          LineaRollupContractVersion.V9 -> // just ensure not regression while WIP
>>>>>>> 5c0bae8ac (chore(coordinator): adds initial scafold for contract v9 RISC-V (#3618)):jvm-libs/linea/clients/linea-contract-clients/src/main/kotlin/linea/contract/l1/Web3JLineaRollupSmartContractClientReadOnly.kt
=======
          LinethRollupContractVersion.V9 -> // just ensure not regression while WIP
>>>>>>> 53054e4e0 (chore(coordinator): rename LineaRollup to LinethRollup in JVM components (#3421))
            contractClientV9AtBlock(blockParameter).blobShnarfExists(shnarf)
        }
          .sendAsync()
          .thenApply { it != BigInteger.ZERO }
          .toSafeFuture()
      }
  }

  override fun blockStateRootHash(blockParameter: BlockParameter, lineaL2BlockNumber: ULong): SafeFuture<ByteArray> {
    return contractClientV8AtBlock(blockParameter)
      .stateRootHashes(lineaL2BlockNumber.toBigInteger()).sendAsync()
      .toSafeFuture()
  }

  override fun getLatestFinalizedState(blockParameter: BlockParameter): SafeFuture<LinethRollupFinalizedState> {
    return getVersion()
      .thenApply { contractVersion ->
        if (contractVersion != LinethRollupContractVersion.V8) {
          throw UnsupportedOperationException("Contract $contractVersion does not support getLatestFinalizedState")
        }
        contractClientV8AtBlock(blockParameter)
      }.thenCompose { contractClient ->
        contractClient
          .currentL2BlockNumber()
          .sendAsync()
          .toSafeFuture()
          .thenCompose { finalizedBlockNumber ->
            getFinalizedStateEvent(
              upToBlock = blockParameter,
              finalisedBlockNumber = finalizedBlockNumber.toULong(),
            )
              .thenApply { finalState ->
                LinethRollupFinalizedState(
                  blockNumber = finalState.blockNumber,
                  blockTimestamp = finalState.timestamp,
                  messageNumber = finalState.messageNumber,
                  forcedTransactionNumber = finalState.forcedTransactionNumber,
                )
              }
          }
      }
  }

  override fun getFinalizedStateData(
    blockParameter: BlockParameter,
  ): SafeFuture<FinalizedStateDataProvider.FinalizedStateData> {
    return getVersion()
      .thenCombine(
        finalizedL2BlockNumber(blockParameter),
      ) { version, finalizedBlockNumber -> version to finalizedBlockNumber }
      .thenCompose { (version, finalizedBlockNumber) ->
        when (version) {
          LinethRollupContractVersion.V6,
          LinethRollupContractVersion.V7,
          ->
            SafeFuture.completedFuture(
              FinalizedStateDataProvider.FinalizedStateData(
                blockNumber = finalizedBlockNumber,
                forcedTransactionNumber = 0UL, // ftx is not available in V6 and V7 contract, return the initial value
              ),
            )

<<<<<<< HEAD
<<<<<<< HEAD:jvm-libs/linea/clients/linea-contract-clients/src/main/kotlin/linea/contract/l1/Web3JLinethRollupSmartContractClientReadOnly.kt
<<<<<<< HEAD:jvm-libs/linea/clients/linea-contract-clients/src/main/kotlin/linea/contract/l1/Web3JLinethRollupSmartContractClientReadOnly.kt
          LinethRollupContractVersion.V8,
          LinethRollupContractVersion.V9,
=======
          LineaRollupContractVersion.V8,
          LineaRollupContractVersion.V9,
>>>>>>> 5c0bae8ac (chore(coordinator): adds initial scafold for contract v9 RISC-V (#3618)):jvm-libs/linea/clients/linea-contract-clients/src/main/kotlin/linea/contract/l1/Web3JLineaRollupSmartContractClientReadOnly.kt
=======
          LineaRollupContractVersion.V8,
          LineaRollupContractVersion.V9,
>>>>>>> 5c0bae8ac (chore(coordinator): adds initial scafold for contract v9 RISC-V (#3618)):jvm-libs/linea/clients/linea-contract-clients/src/main/kotlin/linea/contract/l1/Web3JLineaRollupSmartContractClientReadOnly.kt
=======
          LinethRollupContractVersion.V8,
          LinethRollupContractVersion.V9,
>>>>>>> 53054e4e0 (chore(coordinator): rename LineaRollup to LinethRollup in JVM components (#3421))
          ->
            findFinalizedStateEvent(blockParameter, finalizedBlockNumber)
              .thenApply { finalizedState ->
                val forcedTransactionNumber = finalizedState?.forcedTransactionNumber ?: 0UL
                FinalizedStateDataProvider.FinalizedStateData(
                  blockNumber = finalizedBlockNumber,
                  forcedTransactionNumber = forcedTransactionNumber,
                )
              }
        }
      }
  }

  // The finalized L2 block number increases monotonically, so each FinalizedStateUpdated event is at
  // an L1 block >= the previous one. We remember the last found L1 block and start the next search
  // there, turning the repeated finalization-monitor lookups (polled every ~500ms) into a small
  // forward search instead of a full-chain binary search. A full-range fallback covers the rare case
  // where the cached window overshoots (e.g. an L1 reorg moved the event earlier, or a caller queries
  // a historical block).
  private val finalizedStateSearchFromBlock = AtomicReference<BlockParameter>(
    finalizedStateSearchInitialBlockParameter,
  )

  internal fun findFinalizedStateEvent(
    upToBlock: BlockParameter,
    finalizedBlockNumber: ULong,
  ): SafeFuture<FinalizedStateUpdatedEvent?> {
    val fromBlock = finalizedStateSearchFromBlock.get()
    val search =
      if (fromBlock == BlockParameter.Tag.EARLIEST) {
        searchFinalizedStateEvent(BlockParameter.Tag.EARLIEST, upToBlock, finalizedBlockNumber)
      } else {
        searchFinalizedStateEvent(fromBlock, upToBlock, finalizedBlockNumber)
          // The cached forward window is only an optimization: on a miss OR any failure (e.g. it sits
          // after a historical upToBlock, which EthLogsSearcher rejects as an invalid range) fall back
          // to the authoritative full-range search, which re-surfaces any genuine (e.g. RPC) error.
          .exceptionally { null }
          .thenCompose { event ->
            if (event != null) {
              SafeFuture.completedFuture(event)
            } else {
              searchFinalizedStateEvent(finalizedStateSearchInitialBlockParameter, upToBlock, finalizedBlockNumber)
            }
          }
      }
    return search.thenApply { event ->
      event?.also { finalizedStateSearchFromBlock.set(it.log.blockNumber.toBlockParameter()) }
        ?.event
    }
  }

  private fun searchFinalizedStateEvent(
    fromBlock: BlockParameter,
    upToBlock: BlockParameter,
    finalizedBlockNumber: ULong,
  ): SafeFuture<EthLogEvent<FinalizedStateUpdatedEvent>?> {
    // FinalizedStateUpdated has a unique, monotonically-increasing indexed blockNumber, so we binary
    // search for it in bounded chunks instead of an unbounded getLogs(EARLIEST..upToBlock) query,
    // which rate-limited providers (e.g. Infura) reject for spans > 10_000 blocks.
    return ethLogsSearcher.findLog(
      fromBlock = fromBlock,
      toBlock = upToBlock,
      chunkSize = l1EventSearchMaxBlockRange.toInt(),
      address = contractAddress,
      topics = listOf(FinalizedStateUpdatedEvent.topic),
    ) { ethLog ->
      val foundBlockNumber = FinalizedStateUpdatedEvent.fromEthLog(ethLog).event.blockNumber
      when {
        foundBlockNumber < finalizedBlockNumber -> SearchDirection.FORWARD
        foundBlockNumber > finalizedBlockNumber -> SearchDirection.BACKWARD
        else -> null
      }
    }.thenApply { ethLog ->
      ethLog?.let(FinalizedStateUpdatedEvent::fromEthLog)
    }
  }

  private fun getFinalizedStateEvent(
    upToBlock: BlockParameter,
    finalisedBlockNumber: ULong,
  ): SafeFuture<FinalizedStateUpdatedEvent> {
    return findFinalizedStateEvent(upToBlock = upToBlock, finalizedBlockNumber = finalisedBlockNumber)
      .thenApply { event ->
        // it means contract was just upgraded but no event published yet,
        // we cannot deterministically get the finalized fields
        // throw unsupported operation exception to let caller decide what to do, either retry later or fail
        event ?: throw UnsupportedOperationException("event FinalizedStateUpdated not found on L1")
      }
  }
}
