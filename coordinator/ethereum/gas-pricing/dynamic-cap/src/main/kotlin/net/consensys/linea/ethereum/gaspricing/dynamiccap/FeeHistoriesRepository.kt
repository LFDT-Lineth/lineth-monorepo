package net.consensys.linea.ethereum.gaspricing.dynamiccap

import linea.domain.FeeHistory
import tech.pegasys.teku.infrastructure.async.SafeFuture

interface FeeHistoriesRepository {
  fun saveNewFeeHistory(feeHistory: FeeHistory): SafeFuture<Unit>

  fun findBaseFeePerGasAtPercentile(
    percentile: Double,
    fromBlockNumber: Long,
  ): SafeFuture<ULong?>

  fun findBaseFeePerBlobGasAtPercentile(
    percentile: Double,
    fromBlockNumber: Long,
  ): SafeFuture<ULong?>

  fun findAverageRewardAtPercentile(
    rewardPercentile: Double,
    fromBlockNumber: Long,
  ): SafeFuture<ULong?>

  fun findHighestBlockNumberWithPercentile(
    rewardPercentile: Double,
  ): SafeFuture<Long?>

  fun getNumOfFeeHistoriesFromBlockNumber(
    rewardPercentile: Double,
    fromBlockNumber: Long,
  ): SafeFuture<Int>

  fun deleteFeeHistoriesUpToBlockNumber(
    blockNumberInclusive: Long,
  ): SafeFuture<Int>
}
