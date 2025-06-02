package net.consensys.linea.ethereum.gaspricing.dynamiccap

import io.vertx.junit5.Timeout
import io.vertx.junit5.VertxExtension
import io.vertx.junit5.VertxTestContext
import linea.web3j.EthFeeHistoryBlobExtended
import linea.web3j.Web3jBlobExtended
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.assertThrows
import org.junit.jupiter.api.extension.ExtendWith
import org.mockito.Mockito
import org.mockito.kotlin.any
import org.mockito.kotlin.eq
import org.mockito.kotlin.mock
import org.mockito.kotlin.whenever
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.TimeUnit

@ExtendWith(VertxExtension::class)
class GasPriceCapFeeHistoryFetcherImplTest {
  private val maxBlockCount = 10u
  private val rewardPercentiles = listOf(10.0, 20.0, 30.0, 40.0, 50.0, 60.0, 70.0, 80.0, 90.0, 100.0)

  @Test
  @Timeout(10, timeUnit = TimeUnit.SECONDS)
  fun getEthFeeHistoryData_returnsFeeHistoryData(testContext: VertxTestContext) {
    val feeHistoryFetcherImpl = createFeeHistoryFetcherImpl(
      l1Web3jServiceMock = createMockedWeb3jBlobExtended(),
    )

    feeHistoryFetcherImpl.getEthFeeHistoryData(
      startBlockNumberInclusive = 101L,
      endBlockNumberInclusive = 110L,
    )
      .thenApply {
        testContext
          .verify {
            assertThat(it).isNotNull
            assertThat(it!!.baseFeePerGas.size.toUInt()).isEqualTo(maxBlockCount + 1u)
            assertThat(it.reward.size.toUInt()).isEqualTo(maxBlockCount)
            assertThat(it.gasUsedRatio.size.toUInt()).isEqualTo(maxBlockCount)
            assertThat(it.baseFeePerBlobGas.size.toUInt()).isEqualTo(maxBlockCount + 1u)
            assertThat(it.blobGasUsedRatio.size.toUInt()).isEqualTo(maxBlockCount)
            it.reward.forEach {
              assertThat(it.size).isEqualTo(rewardPercentiles.size)
            }
          }
          .completeNow()
      }
  }

  @Test
  fun getEthFeeHistoryData_returnsFeeHistoryDataWithEmptyBlobData(testContext: VertxTestContext) {
    val feeHistoryFetcherImpl = createFeeHistoryFetcherImpl(
      l1Web3jServiceMock = createMockedWeb3jBlobExtendedWithoutBlobData(),
    )

    feeHistoryFetcherImpl.getEthFeeHistoryData(
      startBlockNumberInclusive = 101L,
      endBlockNumberInclusive = 110L,
    )
      .thenApply {
        testContext
          .verify {
            assertThat(it).isNotNull
            assertThat(it!!.blobGasUsedRatio).isNotNull
            assertThat(it.baseFeePerGas.size.toUInt()).isEqualTo(maxBlockCount + 1u)
            assertThat(it.reward.size.toUInt()).isEqualTo(maxBlockCount)
            assertThat(it.gasUsedRatio.size.toUInt()).isEqualTo(maxBlockCount)
            assertThat(it.baseFeePerBlobGas).isEmpty()
            assertThat(it.blobGasUsedRatio).isEmpty()
            it.reward.forEach {
              assertThat(it.size).isEqualTo(rewardPercentiles.size)
            }
          }
          .completeNow()
      }
  }

  @Test
  @Timeout(10, timeUnit = TimeUnit.SECONDS)
  fun getEthFeeHistoryData_throwsErrorIfEmptyRewardPercentiles(testContext: VertxTestContext) {
    val l1Web3jServiceMock = createMockedWeb3jBlobExtended()

    testContext.verify {
      assertThrows<IllegalArgumentException> {
        GasPriceCapFeeHistoryFetcherImpl(
          l1Web3jServiceMock,
          GasPriceCapFeeHistoryFetcherImpl.Config(
            maxBlockCount,
            rewardPercentiles = listOf(),
          ),
        )
      }.also { exception ->
        assertThat(exception.message)
          .isEqualTo(
            "Reward percentiles must be a non-empty list.",
          )
      }
    }.completeNow()
  }

  @Test
  @Timeout(10, timeUnit = TimeUnit.SECONDS)
  fun getEthFeeHistoryData_throwsErrorIfInvalidRewardPercentiles(testContext: VertxTestContext) {
    val l1Web3jServiceMock = createMockedWeb3jBlobExtended()

    testContext.verify {
      assertThrows<IllegalArgumentException> {
        GasPriceCapFeeHistoryFetcherImpl(
          l1Web3jServiceMock,
          GasPriceCapFeeHistoryFetcherImpl.Config(
            maxBlockCount,
            rewardPercentiles = listOf(101.0, -12.2),
          ),
        )
      }.also { exception ->
        assertThat(exception.message)
          .isEqualTo(
            "Reward percentile must be within 0.0 and 100.0." + " Value=101.0",
          )
      }

      assertThrows<IllegalArgumentException> {
        GasPriceCapFeeHistoryFetcherImpl(
          l1Web3jServiceMock,
          GasPriceCapFeeHistoryFetcherImpl.Config(
            maxBlockCount,
            rewardPercentiles = listOf(-12.2, 1000.0),
          ),
        )
      }.also { exception ->
        assertThat(exception.message)
          .isEqualTo(
            "Reward percentile must be within 0.0 and 100.0." + " Value=-12.2",
          )
      }
    }.completeNow()
  }

  @Test
  @Timeout(10, timeUnit = TimeUnit.SECONDS)
  fun getEthFeeHistoryData_throwsErrorIfStartBlockIsHigherThanTargetBlock(testContext: VertxTestContext) {
    val feeHistoryFetcherImpl = createFeeHistoryFetcherImpl(
      l1Web3jServiceMock = createMockedWeb3jBlobExtended(),
    )

    testContext.verify {
      assertThrows<IllegalArgumentException> {
        feeHistoryFetcherImpl.getEthFeeHistoryData(
          startBlockNumberInclusive = 111L,
          endBlockNumberInclusive = 100L,
        ).get()
      }.also { exception ->
        assertThat(exception.message)
          .isEqualTo(
            "endBlockNumberInclusive=100 must be equal or higher than startBlockNumberInclusive=111",
          )
      }
    }.completeNow()
  }

  @Test
  @Timeout(10, timeUnit = TimeUnit.SECONDS)
  fun getEthFeeHistoryData_throwsErrorIfBlockDiffIsLargerThanMaxBlockCount(testContext: VertxTestContext) {
    val feeHistoryFetcherImpl = createFeeHistoryFetcherImpl(
      l1Web3jServiceMock = createMockedWeb3jBlobExtended(),
    )

    testContext.verify {
      assertThrows<IllegalArgumentException> {
        feeHistoryFetcherImpl.getEthFeeHistoryData(
          startBlockNumberInclusive = 101L,
          endBlockNumberInclusive = 120L,
        ).get()
      }.also { exception ->
        assertThat(exception.message)
          .isEqualTo(
            "difference between endBlockNumberInclusive=120 and startBlockNumberInclusive=101 " +
              "must be less than maxBlockCount=10",
          )
      }
    }.completeNow()
  }

  private fun createMockedWeb3jBlobExtended(): Web3jBlobExtended {
    val web3jService = mock<Web3jBlobExtended>(defaultAnswer = Mockito.RETURNS_DEEP_STUBS)
    whenever(
      web3jService
        .ethFeeHistoryWithBlob(
          any(),
          any(),
          eq(rewardPercentiles),
        )
        .sendAsync(),
    )
      .thenAnswer {
        val feeHistoryResponse = EthFeeHistoryBlobExtended()
        val feeHistory = EthFeeHistoryBlobExtended.FeeHistoryBlobExtended(
          oldestBlock = "0x16",
          reward = (1000 until 1000 + maxBlockCount.toLong())
            .map { reward -> (1..rewardPercentiles.size).map { reward.toString() } },
          baseFeePerGas = (10000 until 10000 + maxBlockCount.toLong() + 1)
            .map { it.toString() },
          gasUsedRatio = (10 until 10 + maxBlockCount.toLong())
            .map { it / 100.0 },
          baseFeePerBlobGas = (10000 until 10000 + maxBlockCount.toLong() + 1)
            .map { it.toString() },
          blobGasUsedRatio = (10 until 10 + maxBlockCount.toLong())
            .map { it / 100.0 },
        )
        feeHistoryResponse.result = feeHistory
        SafeFuture.completedFuture(feeHistoryResponse)
      }

    return web3jService
  }

  private fun createMockedWeb3jBlobExtendedWithoutBlobData(): Web3jBlobExtended {
    val web3jService = mock<Web3jBlobExtended>(defaultAnswer = Mockito.RETURNS_DEEP_STUBS)
    whenever(
      web3jService
        .ethFeeHistoryWithBlob(
          any(),
          any(),
          eq(rewardPercentiles),
        )
        .sendAsync(),
    )
      .thenAnswer {
        val feeHistoryResponse = EthFeeHistoryBlobExtended()
        val feeHistory = EthFeeHistoryBlobExtended.FeeHistoryBlobExtended(
          oldestBlock = "0x16",
          reward = (1000 until 1000 + maxBlockCount.toLong())
            .map { reward -> (1..rewardPercentiles.size).map { reward.toString() } },
          baseFeePerGas = (10000 until 10000 + maxBlockCount.toLong() + 1)
            .map { it.toString() },
          gasUsedRatio = (10 until 10 + maxBlockCount.toLong())
            .map { it / 100.0 },
          baseFeePerBlobGas = emptyList(),
          blobGasUsedRatio = emptyList(),
        )
        feeHistoryResponse.result = feeHistory
        SafeFuture.completedFuture(feeHistoryResponse)
      }

    return web3jService
  }

  private fun createFeeHistoryFetcherImpl(
    l1Web3jServiceMock: Web3jBlobExtended,
  ): GasPriceCapFeeHistoryFetcherImpl {
    return GasPriceCapFeeHistoryFetcherImpl(
      l1Web3jServiceMock,
      GasPriceCapFeeHistoryFetcherImpl.Config(
        maxBlockCount,
        rewardPercentiles,
      ),
    )
  }
}
