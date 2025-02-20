package net.consensys.zkevm.persistence.dao.feehistory

import linea.domain.FeeHistory
import net.consensys.linea.ethereum.gaspricing.dynamiccap.FeeHistoriesRepositoryWithCache
import net.consensys.linea.ethereum.gaspricing.dynamiccap.PercentileGasFees
import net.consensys.zkevm.persistence.db.DuplicatedRecordException
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.atomic.AtomicInteger
import java.util.concurrent.atomic.AtomicReference

class FeeHistoriesRepositoryImpl(
  private val config: Config,
  private val feeHistoriesDao: FeeHistoriesDao
) : FeeHistoriesRepositoryWithCache {
  data class Config(
    val rewardPercentiles: List<Double>,
    val minBaseFeePerBlobGasToCache: ULong? = null,
    val fixedAverageRewardToCache: ULong? = null
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

  private var lastNumOfFeeHistoriesFromBlockNumber: AtomicInteger = AtomicInteger(0)
  private var lastPercentileGasFees: AtomicReference<PercentileGasFees> =
    AtomicReference(
      PercentileGasFees(
        percentileBaseFeePerGas = 0uL,
        percentileBaseFeePerBlobGas = 0uL,
        percentileAvgReward = 0uL
      )
    )

  override fun saveNewFeeHistory(feeHistory: FeeHistory): SafeFuture<Unit> {
    return feeHistoriesDao.saveNewFeeHistory(
      feeHistory,
      config.rewardPercentiles
    )
      .exceptionallyCompose { error ->
        if (error is DuplicatedRecordException) {
          SafeFuture.completedFuture(Unit)
        } else {
          SafeFuture.failedFuture(error)
        }
      }
  }

  override fun findBaseFeePerGasAtPercentile(
    percentile: Double,
    fromBlockNumber: Long
  ): SafeFuture<ULong?> {
    return feeHistoriesDao.findBaseFeePerGasAtPercentile(
      percentile,
      fromBlockNumber
    )
  }

  override fun findBaseFeePerBlobGasAtPercentile(
    percentile: Double,
    fromBlockNumber: Long
  ): SafeFuture<ULong?> {
    return feeHistoriesDao.findBaseFeePerBlobGasAtPercentile(
      percentile,
      fromBlockNumber
    )
  }

  override fun findAverageRewardAtPercentile(
    rewardPercentile: Double,
    fromBlockNumber: Long
  ): SafeFuture<ULong?> {
    return feeHistoriesDao.findAverageRewardAtPercentile(
      rewardPercentile,
      fromBlockNumber
    )
  }

  override fun findHighestBlockNumberWithPercentile(rewardPercentile: Double): SafeFuture<Long?> {
    return feeHistoriesDao.findHighestBlockNumberWithPercentile(rewardPercentile)
  }

  override fun getNumOfFeeHistoriesFromBlockNumber(
    rewardPercentile: Double,
    fromBlockNumber: Long
  ): SafeFuture<Int> {
    return feeHistoriesDao.getNumOfFeeHistoriesFromBlockNumber(
      rewardPercentile,
      fromBlockNumber
    )
  }

  override fun getCachedNumOfFeeHistoriesFromBlockNumber(): Int {
    return lastNumOfFeeHistoriesFromBlockNumber.get()
  }

  override fun cacheNumOfFeeHistoriesFromBlockNumber(
    rewardPercentile: Double,
    fromBlockNumber: Long
  ): SafeFuture<Int> {
    return feeHistoriesDao.getNumOfFeeHistoriesFromBlockNumber(
      rewardPercentile,
      fromBlockNumber
    ).thenPeek {
      lastNumOfFeeHistoriesFromBlockNumber.set(it)
    }
  }

  override fun getCachedPercentileGasFees(): PercentileGasFees {
    return lastPercentileGasFees.get()
  }

  override fun cachePercentileGasFees(
    percentile: Double,
    fromBlockNumber: Long
  ): SafeFuture<Unit> {
    return findBaseFeePerGasAtPercentile(
      percentile,
      fromBlockNumber
    ).thenCompose { percentileBaseFeePerGas ->
      findBaseFeePerBlobGasAtPercentile(
        percentile,
        fromBlockNumber
      ).thenCompose { percentileBaseFeePerBlobGas ->
        (
          if (config.fixedAverageRewardToCache != null) {
            SafeFuture.completedFuture(config.fixedAverageRewardToCache)
          } else {
            findAverageRewardAtPercentile(percentile, fromBlockNumber)
          }
          ).thenApply { percentileAvgReward ->
          lastPercentileGasFees.set(
            PercentileGasFees(
              percentileBaseFeePerGas!!,
              percentileBaseFeePerBlobGas!!
                .coerceAtLeast(config.minBaseFeePerBlobGasToCache ?: percentileBaseFeePerBlobGas),
              percentileAvgReward!!
            )
          )
        }
      }
    }
  }

  override fun deleteFeeHistoriesUpToBlockNumber(
    blockNumberInclusive: Long
  ): SafeFuture<Int> {
    return feeHistoriesDao.deleteFeeHistoriesUpToBlockNumber(blockNumberInclusive)
  }
}
