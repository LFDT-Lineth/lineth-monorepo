package net.consensys.linea.ethereum.gaspricing.dynamiccap

import linea.domain.BlockParameter
import linea.domain.BlockParameter.Companion.toBlockParameter
import linea.domain.FeeHistory
import linea.ethapi.EthApiFeeClient
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture

class GasPriceCapFeeHistoryFetcherImpl(
  private val ethApiFeeClient: EthApiFeeClient,
  private val config: Config,
) : GasPriceCapFeeHistoryFetcher {
  private val log: Logger = LogManager.getLogger(this::class.java)

  data class Config(
    val maxBlockCount: UInt,
    val rewardPercentiles: List<Double>,
  ) {
    init {
      require(rewardPercentiles.isNotEmpty()) {
        "Reward percentiles must be a non-empty list."
      }

      rewardPercentiles.forEach { percentile ->
        require(percentile in 0.0..100.0) {
          "Reward percentile must be within 0.0 and 100.0." +
            " Value=$percentile"
        }
      }
    }
  }

  private fun getFeeHistory(newestBlock: BlockParameter, blockCount: Int): SafeFuture<FeeHistory> {
    return SafeFuture.of(
      ethApiFeeClient.ethFeeHistory(
        blockCount,
        newestBlock,
        config.rewardPercentiles,
      ),
    )
  }

  override fun getEthFeeHistoryData(
    startBlockNumberInclusive: Long,
    endBlockNumberInclusive: Long,
  ): SafeFuture<FeeHistory> {
    require(endBlockNumberInclusive >= startBlockNumberInclusive) {
      "endBlockNumberInclusive=$endBlockNumberInclusive must be equal or higher " +
        "than startBlockNumberInclusive=$startBlockNumberInclusive"
    }

    require(endBlockNumberInclusive - startBlockNumberInclusive < config.maxBlockCount.toLong()) {
      "difference between endBlockNumberInclusive=$endBlockNumberInclusive and " +
        "startBlockNumberInclusive=$startBlockNumberInclusive must be less " +
        "than maxBlockCount=${config.maxBlockCount}"
    }

    val newestBlock = endBlockNumberInclusive.toBlockParameter()
    val blockCount = (endBlockNumberInclusive - startBlockNumberInclusive).inc().toInt()

    log.debug(
      "Fetching fee history data: startBlockNumberInclusive={} " +
        "endBlockNumberInclusive={} newestBlock={} blockCount={}",
      startBlockNumberInclusive,
      endBlockNumberInclusive,
      newestBlock.getNumber(),
      blockCount,
    )

    return getFeeHistory(newestBlock, blockCount)
      .whenException { th ->
        log.warn("Get fee history data failure: errorMessage={}", th.message, th)
      }
  }
}
