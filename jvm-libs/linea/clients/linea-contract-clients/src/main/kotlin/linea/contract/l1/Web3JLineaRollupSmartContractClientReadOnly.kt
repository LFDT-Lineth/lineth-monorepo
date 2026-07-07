package linea.contract.l1

import linea.EthLogsSearcher
import linea.SearchDirection
import linea.contract.FAKE_READ_ONLY_CREDENTIALS
import linea.contract.LineaRollupV6
import linea.contract.LineaRollupV8
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

open class Web3JLineaRollupSmartContractClientReadOnly(
  val web3j: Web3j,
  val contractAddress: String,
  private val ethLogsSearcher: EthLogsSearcher,
  private val l1EventSearchMaxBlockRange: UInt = 10_000u,
  private val log: Logger = LogManager.getLogger(Web3JLineaRollupSmartContractClientReadOnly::class.java),
) : LineaRollupSmartContractClientReadOnly,
  LineaRollupSmartContractClientReadOnlyFinalizedStateProvider,
  FinalizedStateDataProvider {

  protected fun contractClientV8AtBlock(blockParameter: BlockParameter): LineaRollupV8 {
    return contractClientAtBlock(blockParameter, LineaRollupV8::class.java)
  }

  protected fun <T : Contract> loadContractClient(contract: Class<T>): T {
    @Suppress("UNCHECKED_CAST")
    return when {
      LineaRollupV6::class.java.isAssignableFrom(contract) -> LineaRollupV6.load(
        contractAddress,
        web3j,
        FAKE_READ_ONLY_CREDENTIALS,
        StaticGasProvider(BigInteger.ZERO, BigInteger.ZERO),
      )

      LineaRollupV8::class.java.isAssignableFrom(contract) -> LineaRollupV8.load(
        contractAddress,
        web3j,
        FAKE_READ_ONLY_CREDENTIALS,
        StaticGasProvider(BigInteger.ZERO, BigInteger.ZERO),
      )

      else -> throw IllegalArgumentException("Unsupported contract type: ${contract::class.java}")
    } as T
  }

  protected fun <T : Contract> contractClientAtBlock(blockParameter: BlockParameter, contract: Class<T>): T {
    @Suppress("UNCHECKED_CAST")
    return loadContractClient(contract).apply {
      this.setDefaultBlockParameter(blockParameter.toWeb3j())
    }
  }

  private val smartContractVersionCache = AtomicReference<LineaRollupContractVersion>(null)

  private fun getSmartContractVersion(): SafeFuture<LineaRollupContractVersion> {
    return if (smartContractVersionCache.get() == LineaRollupContractVersion.V8) {
      // once upgraded, it's not downgraded
      SafeFuture.completedFuture(LineaRollupContractVersion.V8)
    } else {
      fetchSmartContractVersion()
        .thenPeek { contractLatestVersion ->
          if (smartContractVersionCache.get() != null &&
            contractLatestVersion != smartContractVersionCache.get()
          ) {
            log.info(
              "Smart contract upgraded: prevVersion={} upgradedVersion={}",
              smartContractVersionCache.get(),
              contractLatestVersion,
            )
          }
          smartContractVersionCache.set(contractLatestVersion)
        }
    }
  }

  private fun fetchSmartContractVersion(
    blockParameter: BlockParameter = BlockParameter.Tag.LATEST,
  ): SafeFuture<LineaRollupContractVersion> {
    return contractClientAtBlock(blockParameter, LineaRollupV6::class.java)
      .CONTRACT_VERSION()
      .sendAsync()
      .toSafeFuture()
      .thenApply(::parseContractVersion)
  }

  private fun parseContractVersion(version: String): LineaRollupContractVersion {
    return when {
      version.startsWith("6") -> LineaRollupContractVersion.V6
      version.startsWith("7") -> LineaRollupContractVersion.V7
      version.startsWith("8") -> LineaRollupContractVersion.V8
      else -> throw IllegalStateException("Unsupported contract version: $version")
    }
  }

  override fun getAddress(): String = contractAddress

  override fun getVersion(blockParameter: BlockParameter): SafeFuture<LineaRollupContractVersion> {
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
          LineaRollupContractVersion.V6,
          LineaRollupContractVersion.V7,
          LineaRollupContractVersion.V8,
          -> contractClientV8AtBlock(blockParameter).blobShnarfExists(shnarf)
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

  override fun getLatestFinalizedState(blockParameter: BlockParameter): SafeFuture<LineaRollupFinalizedState> {
    return getVersion()
      .thenApply { contractVersion ->
        if (contractVersion != LineaRollupContractVersion.V8) {
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
                LineaRollupFinalizedState(
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
          LineaRollupContractVersion.V6,
          LineaRollupContractVersion.V7,
          ->
            SafeFuture.completedFuture(
              FinalizedStateDataProvider.FinalizedStateData(
                blockNumber = finalizedBlockNumber,
                forcedTransactionNumber = 0UL, // ftx is not available in V6 and V7 contract, return the initial value
              ),
            )

          LineaRollupContractVersion.V8 ->
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
  private val finalizedStateSearchFromBlock = AtomicReference<BlockParameter>(BlockParameter.Tag.EARLIEST)

  internal fun findFinalizedStateEvent(
    upToBlock: BlockParameter,
    finalisedBlockNumber: ULong,
  ): SafeFuture<FinalizedStateUpdatedEvent?> {
    val fromBlock = finalizedStateSearchFromBlock.get()
    val search =
      if (fromBlock == BlockParameter.Tag.EARLIEST) {
        searchFinalizedStateEvent(BlockParameter.Tag.EARLIEST, upToBlock, finalisedBlockNumber)
      } else {
        searchFinalizedStateEvent(fromBlock, upToBlock, finalisedBlockNumber)
          // The cached forward window is only an optimization: on a miss OR any failure (e.g. it sits
          // after a historical upToBlock, which EthLogsSearcher rejects as an invalid range) fall back
          // to the authoritative full-range search, which re-surfaces any genuine (e.g. RPC) error.
          .exceptionally { null }
          .thenCompose { event ->
            if (event != null) {
              SafeFuture.completedFuture(event)
            } else {
              searchFinalizedStateEvent(BlockParameter.Tag.EARLIEST, upToBlock, finalisedBlockNumber)
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
    finalisedBlockNumber: ULong,
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
        foundBlockNumber < finalisedBlockNumber -> SearchDirection.FORWARD
        foundBlockNumber > finalisedBlockNumber -> SearchDirection.BACKWARD
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
    return findFinalizedStateEvent(upToBlock = upToBlock, finalisedBlockNumber = finalisedBlockNumber)
      .thenApply { event ->
        // it means contract was just upgraded but no event published yet,
        // we cannot deterministically get the finalized fields
        // throw unsupported operation exception to let caller decide what to do, either retry later or fail
        event ?: throw UnsupportedOperationException("event FinalizedStateUpdated not found on L1")
      }
  }
}
