package net.consensys.linea.ethereum.gaspricing.staticcap

import net.consensys.linea.FeeHistory
import net.consensys.linea.ethereum.gaspricing.FeesFetcher
import net.consensys.linea.web3j.Web3jBlobExtended
import net.consensys.toIntervalString
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.web3j.protocol.Web3j
import org.web3j.protocol.core.DefaultBlockParameterName
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.math.BigInteger

class FeeHistoryFetcherImpl(
  private val web3jClient: Web3j,
  private val web3jService: Web3jBlobExtended,
  private val config: Config
) : FeesFetcher {
  private val log: Logger = LogManager.getLogger(this::class.java)

  data class Config(
    val feeHistoryBlockCount: UInt,
    val feeHistoryRewardPercentile: Double
  ) {
    init {
      require(feeHistoryRewardPercentile in 0.0..100.0) {
        "feeHistoryRewardPercentile must be within 0.0 and 100.0." +
          " Value=$feeHistoryRewardPercentile"
      }
    }
  }

  private var cacheIsValidForBlockNumber: BigInteger = BigInteger.ZERO
  private lateinit var feesCache: FeeHistory

  private fun getRecentFees(): SafeFuture<FeeHistory> {
    val blockNumberFuture = web3jClient.ethBlockNumber().sendAsync()
    return SafeFuture.of(blockNumberFuture)
      .thenCompose { blockNumberResponse ->
        val currentBlockNumber = blockNumberResponse.blockNumber
        if (currentBlockNumber > cacheIsValidForBlockNumber) {
          web3jService
            .ethFeeHistoryWithBlob(
              config.feeHistoryBlockCount.toInt(),
              DefaultBlockParameterName.LATEST,
              listOf(config.feeHistoryRewardPercentile)
            )
            .sendAsync()
            .thenApply {
              val feeHistory = it.feeHistory.toLineaDomain()
              cacheIsValidForBlockNumber = currentBlockNumber
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
                  feeHistory.blobGasUsedRatio
                )
              } else {
                log.trace(
                  "New Fee History: l1BlockNumber={} l1Blocks={} lastBaseFeePerGas={} reward={} gasUsedRatio={}",
                  currentBlockNumber,
                  feeHistory.blocksRange().toIntervalString(),
                  feeHistory.baseFeePerGas[feeHistory.baseFeePerGas.lastIndex],
                  feeHistory.reward.map { percentiles -> percentiles[0] },
                  feeHistory.gasUsedRatio
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
