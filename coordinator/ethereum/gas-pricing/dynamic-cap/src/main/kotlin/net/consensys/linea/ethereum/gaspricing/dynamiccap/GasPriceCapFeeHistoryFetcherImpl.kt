package net.consensys.linea.ethereum.gaspricing.dynamiccap

import linea.domain.FeeHistory
import linea.web3j.Web3jBlobExtended
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.web3j.protocol.core.DefaultBlockParameter
import org.web3j.protocol.core.DefaultBlockParameterNumber
import tech.pegasys.teku.infrastructure.async.SafeFuture

class GasPriceCapFeeHistoryFetcherImpl(
  private val web3jService: Web3jBlobExtended,
  private val config: Config
) : GasPriceCapFeeHistoryFetcher {
  private val log: Logger = LogManager.getLogger(this::class.java)

  data class Config(
    val maxBlockCount: UInt,
    val rewardPercentiles: List<Double>
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

  private fun getFeeHistory(newestBlock: DefaultBlockParameter, blockCount: Int): SafeFuture<FeeHistory> {
    return SafeFuture.of(
      web3jService.ethFeeHistoryWithBlob(
        blockCount,
        newestBlock,
        config.rewardPercentiles
      ).sendAsync()
    ).thenApply {
      if (it.hasError()) {
        throw Exception(it.error.message)
      } else {
        it.feeHistory.toLineaDomain()
      }
    }
  }

  override fun getEthFeeHistoryData(
    startBlockNumberInclusive: Long,
    endBlockNumberInclusive: Long
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

    val newestBlock = DefaultBlockParameterNumber(endBlockNumberInclusive)
    val blockCount = (endBlockNumberInclusive - startBlockNumberInclusive).inc().toInt()

    log.debug(
      "Fetching fee history data: startBlockNumberInclusive={} " +
        "endBlockNumberInclusive={} newestBlock={} blockCount={}",
      startBlockNumberInclusive,
      endBlockNumberInclusive,
      newestBlock.blockNumber,
      blockCount
    )

    return getFeeHistory(newestBlock, blockCount)
      .whenException { th ->
        log.warn("Get fee history data failure: errorMessage={}", th.message, th)
      }
  }
}
