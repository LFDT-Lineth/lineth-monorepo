package net.consensys.linea.ethereum.gaspricing.staticcap

import io.vertx.junit5.Timeout
import io.vertx.junit5.VertxExtension
import io.vertx.junit5.VertxTestContext
import linea.domain.BlockParameter
import linea.domain.FeeHistory
import linea.ethapi.EthApiClient
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import org.mockito.kotlin.eq
import org.mockito.kotlin.mock
import org.mockito.kotlin.whenever
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.TimeUnit

@ExtendWith(VertxExtension::class)
class FeeHistoryFetcherImplTest {
  private val feeHistoryBlockCount = 10u
  private val feeHistoryRewardPercentile = 15.0

  @Test
  @Timeout(10, timeUnit = TimeUnit.SECONDS)
  fun feeHistoryFetcherImpl_returnsFeeHistoryData(testContext: VertxTestContext) {
    val ethApiClient = createMockedEthApiClient()

    val feeHistoryFetcherImpl =
      FeeHistoryFetcherImpl(
        ethApiClient = ethApiClient,
        config = FeeHistoryFetcherImpl.Config(
          feeHistoryBlockCount,
          feeHistoryRewardPercentile,
        ),
      )

    feeHistoryFetcherImpl.getL1EthGasPriceData()
      .thenApply {
        testContext
          .verify {
            assertThat(it).isNotNull
            assertThat(it.baseFeePerGas.size.toUInt()).isEqualTo(feeHistoryBlockCount + 1u)
            assertThat(it.reward.size.toUInt()).isEqualTo(feeHistoryBlockCount)
            assertThat(it.gasUsedRatio.size.toUInt()).isEqualTo(feeHistoryBlockCount)
            assertThat(it.baseFeePerBlobGas.size.toUInt()).isEqualTo(feeHistoryBlockCount + 1u)
            assertThat(it.blobGasUsedRatio.size.toUInt()).isEqualTo(feeHistoryBlockCount)
            it.reward.forEach {
              assertThat(it.size).isEqualTo(1)
            }
          }
          .completeNow()
      }
  }

  @Test
  fun feeHistoryFetcherImpl_returnsFeeHistoryDataWithEmptyBlobData(testContext: VertxTestContext) {
    val ethApiClient = createMockedEthApiClient(feeHistoryWithoutBlobData = true)
    val feeHistoryFetcherImpl =
      FeeHistoryFetcherImpl(
        ethApiClient = ethApiClient,
        config = FeeHistoryFetcherImpl.Config(
          feeHistoryBlockCount,
          feeHistoryRewardPercentile,
        ),
      )

    feeHistoryFetcherImpl.getL1EthGasPriceData()
      .thenApply {
        testContext
          .verify {
            assertThat(it).isNotNull
            assertThat(it.blobGasUsedRatio).isNotNull
            assertThat(it.baseFeePerGas.size.toUInt()).isEqualTo(feeHistoryBlockCount + 1u)
            assertThat(it.reward.size.toUInt()).isEqualTo(feeHistoryBlockCount)
            assertThat(it.gasUsedRatio.size.toUInt()).isEqualTo(feeHistoryBlockCount)
            assertThat(it.baseFeePerBlobGas).isEmpty()
            assertThat(it.blobGasUsedRatio).isEmpty()
            it.reward.forEach {
              assertThat(it.size).isEqualTo(1)
            }
          }
          .completeNow()
      }
  }

  private fun createMockedEthApiClient(feeHistoryWithoutBlobData: Boolean = false): EthApiClient {
    val ethApiClient = mock<EthApiClient>()
    whenever(ethApiClient.ethBlockNumber())
      .thenReturn(SafeFuture.completedFuture(13uL))
    whenever(
      ethApiClient
        .ethFeeHistory(
          eq(feeHistoryBlockCount.toInt()),
          eq(BlockParameter.Tag.LATEST),
          eq(listOf(feeHistoryRewardPercentile)),
        ),
    )
      .thenAnswer {
        val baseFeePerBlobGas = if (!feeHistoryWithoutBlobData) {
          (10000 until 10011).map { it.toULong() }
        } else {
          emptyList()
        }

        val blobGasUsedRatio = if (!feeHistoryWithoutBlobData) {
          (10 until 20).map { it / 100.0 }
        } else {
          emptyList()
        }

        val feeHistory = FeeHistory(
          oldestBlock = 0x16u,
          reward = (1000 until 1010).map { listOf(it.toULong()) },
          baseFeePerGas = (10000 until 10011).map { it.toULong() },
          gasUsedRatio = (10 until 20).map { it / 100.0 },
          baseFeePerBlobGas = baseFeePerBlobGas,
          blobGasUsedRatio = blobGasUsedRatio,
        )
        SafeFuture.completedFuture(feeHistory)
      }
    return ethApiClient
  }
}
