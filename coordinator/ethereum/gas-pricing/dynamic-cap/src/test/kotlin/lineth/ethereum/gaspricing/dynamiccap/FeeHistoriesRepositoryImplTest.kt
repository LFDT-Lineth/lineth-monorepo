package lineth.ethereum.gaspricing.dynamiccap

import lineth.persistence.FeeHistoriesDao
import lineth.domain.FeeHistory
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import tech.pegasys.teku.infrastructure.async.SafeFuture

class FeeHistoriesRepositoryImplTest {
  @Test
  fun `initializes empty cached values`() {
    val repository = FeeHistoriesRepositoryImpl(
      config = FeeHistoriesRepositoryImpl.Config(rewardPercentiles = listOf(50.0)),
      feeHistoriesDao = UnusedFeeHistoriesDao,
    )

    assertThat(repository.getCachedNumOfFeeHistoriesFromBlockNumber()).isZero()
    assertThat(repository.getCachedPercentileGasFees()).isEqualTo(
      PercentileGasFees(
        percentileBaseFeePerGas = 0UL,
        percentileBaseFeePerBlobGas = 0UL,
        percentileAvgReward = 0UL,
      ),
    )
  }

  private object UnusedFeeHistoriesDao : FeeHistoriesDao {
    override fun saveNewFeeHistory(
      feeHistory: FeeHistory,
      rewardPercentiles: List<Double>,
    ): SafeFuture<Unit> = unused()

    override fun findBaseFeePerGasAtPercentile(
      percentile: Double,
      fromBlockNumber: Long,
    ): SafeFuture<ULong?> = unused()

    override fun findBaseFeePerBlobGasAtPercentile(
      percentile: Double,
      fromBlockNumber: Long,
    ): SafeFuture<ULong?> = unused()

    override fun findAverageRewardAtPercentile(
      rewardPercentile: Double,
      fromBlockNumber: Long,
    ): SafeFuture<ULong?> = unused()

    override fun findHighestBlockNumberWithPercentile(
      rewardPercentile: Double,
    ): SafeFuture<Long?> = unused()

    override fun getNumOfFeeHistoriesFromBlockNumber(
      rewardPercentile: Double,
      fromBlockNumber: Long,
    ): SafeFuture<Int> = unused()

    override fun deleteFeeHistoriesUpToBlockNumber(
      blockNumberInclusive: Long,
    ): SafeFuture<Int> = unused()

    private fun unused(): Nothing = error("DAO should not be called")
  }
}
