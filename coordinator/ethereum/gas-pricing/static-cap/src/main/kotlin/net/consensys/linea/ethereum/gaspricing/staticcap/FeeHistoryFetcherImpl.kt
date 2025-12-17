package net.consensys.linea.ethereum.gaspricing.staticcap

import linea.domain.BlockParameter
import linea.domain.FeeHistory
import linea.ethapi.EthApiClient
import linea.kotlin.toIntervalString
import net.consensys.linea.ethereum.gaspricing.FeesFetcher
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.concurrent.atomics.AtomicLong
import kotlin.concurrent.atomics.ExperimentalAtomicApi

@OptIn(ExperimentalAtomicApi::class)
class FeeHistoryFetcherImpl(
  private val ethApiClient: EthApiClient,
  private val config: Config,
) : FeesFetcher {
  private val log: Logger = LogManager.getLogger(this::class.java)

  data class Config(
    val feeHistoryBlockCount: UInt,
    val feeHistoryRewardPercentile: Double,
  ) {
    init {
      require(feeHistoryBlockCount > 0u) {
        "feeHistoryBlockCount=$feeHistoryBlockCount must be greater than 0."
      }
      require(feeHistoryRewardPercentile in 0.0..100.0) {
        "feeHistoryRewardPercentile must be within 0.0 and 100.0." +
          " Value=$feeHistoryRewardPercentile"
      }
    }
  }

  private var cacheIsValidForBlockNumber: AtomicLong = AtomicLong(0)
  private lateinit var feesCache: FeeHistory

  private fun getRecentFees(): SafeFuture<FeeHistory> {
    return ethApiClient.ethBlockNumber()
      .thenCompose { blockNumberResponse ->
        val currentBlockNumber = blockNumberResponse.toLong()
        if (currentBlockNumber > cacheIsValidForBlockNumber.load()) {
          ethApiClient
            .ethFeeHistory(
              blockCount = config.feeHistoryBlockCount.toInt(),
              newestBlock = BlockParameter.Tag.LATEST,
              rewardPercentiles = listOf(config.feeHistoryRewardPercentile),
            )
            .thenApply { feeHistory ->
              cacheIsValidForBlockNumber.store(currentBlockNumber)
              if (feeHistory.baseFeePerBlobGas.isNotEmpty()) {
                log.trace(
                  "New Fee History: l1BlockNumber={} l1Blocks={} lastBaseFeePerGas={} reward={} gasUsedRatio={}" +
                    "baseFeePerBlobGas={} blobGasUsedRatio={}",
                  currentBlockNumber,
                  feeHistory.blocksRange().toIntervalString(),
                  feeHistory.baseFeePerGas[feeHistory.baseFeePerGas.lastIndex],
                  feeHistory.reward.map { percentiles -> percentiles[0] },
                  feeHistory.gasUsedRatio,
                  feeHistory.baseFeePerBlobGas[feeHistory.baseFeePerBlobGas.lastIndex],
                  feeHistory.blobGasUsedRatio,
                )
              } else {
                log.trace(
                  "New Fee History: l1BlockNumber={} l1Blocks={} lastBaseFeePerGas={} reward={} gasUsedRatio={}",
                  currentBlockNumber,
                  feeHistory.blocksRange().toIntervalString(),
                  feeHistory.baseFeePerGas[feeHistory.baseFeePerGas.lastIndex],
                  feeHistory.reward.map { percentiles -> percentiles[0] },
                  feeHistory.gasUsedRatio,
                )
              }
              feesCache = feeHistory
              feesCache
            }
        } else {
          SafeFuture.completedFuture(feesCache)
        }
      }
  }

  override fun getL1EthGasPriceData(): SafeFuture<FeeHistory> {
    return getRecentFees()
      .whenException { th ->
        log.error("Get L1 gas price data failure: {}", th.message, th)
      }
  }
}
