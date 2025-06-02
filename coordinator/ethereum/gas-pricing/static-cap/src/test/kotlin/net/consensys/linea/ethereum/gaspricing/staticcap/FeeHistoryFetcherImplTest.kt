package net.consensys.linea.ethereum.gaspricing.staticcap

import io.vertx.junit5.Timeout
import io.vertx.junit5.VertxExtension
import io.vertx.junit5.VertxTestContext
import linea.web3j.EthFeeHistoryBlobExtended
import linea.web3j.Web3jBlobExtended
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import org.mockito.Mockito
import org.mockito.kotlin.eq
import org.mockito.kotlin.mock
import org.mockito.kotlin.whenever
import org.web3j.protocol.Web3j
import org.web3j.protocol.core.DefaultBlockParameterName
import org.web3j.protocol.core.methods.response.EthBlockNumber
import org.web3j.protocol.core.methods.response.EthGasPrice
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.math.BigInteger
import java.util.concurrent.CompletableFuture
import java.util.concurrent.TimeUnit

@ExtendWith(VertxExtension::class)
class FeeHistoryFetcherImplTest {
  private val feeHistoryBlockCount = 10u
  private val feeHistoryRewardPercentile = 15.0

  @Test
  @Timeout(10, timeUnit = TimeUnit.SECONDS)
  fun feeHistoryFetcherImpl_returnsFeeHistoryData(testContext: VertxTestContext) {
    val l1ClientMock = createMockedWeb3jClient()
    val l1Web3jServiceMock = createMockedWeb3jBlobExtended()

    val feeHistoryFetcherImpl =
      FeeHistoryFetcherImpl(
        l1ClientMock,
        l1Web3jServiceMock,
        FeeHistoryFetcherImpl.Config(
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
    val l1ClientMock = createMockedWeb3jClient()
    val l1Web3jServiceMock = createMockedWeb3jBlobExtendedWithoutBlobData()

    val feeHistoryFetcherImpl =
      FeeHistoryFetcherImpl(
        l1ClientMock,
        l1Web3jServiceMock,
        FeeHistoryFetcherImpl.Config(
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

  private fun createMockedWeb3jClient(): Web3j {
    val web3jClient = mock<Web3j>(defaultAnswer = Mockito.RETURNS_DEEP_STUBS)
    whenever(
      web3jClient
        .ethGasPrice()
        .sendAsync(),
    )
      .thenAnswer {
        val gasPriceResponse = EthGasPrice()
        gasPriceResponse.result = "0x100"
        SafeFuture.completedFuture(gasPriceResponse)
      }
    val mockBlockNumberReturn = mock<EthBlockNumber>()
    whenever(mockBlockNumberReturn.blockNumber).thenReturn(BigInteger.valueOf(13L))
    whenever(web3jClient.ethBlockNumber().sendAsync())
      .thenReturn(CompletableFuture.completedFuture(mockBlockNumberReturn))

    return web3jClient
  }

  private fun createMockedWeb3jBlobExtended(): Web3jBlobExtended {
    val web3jService = mock<Web3jBlobExtended>(defaultAnswer = Mockito.RETURNS_DEEP_STUBS)
    whenever(
      web3jService
        .ethFeeHistoryWithBlob(
          eq(feeHistoryBlockCount.toInt()),
          eq(DefaultBlockParameterName.LATEST),
          eq(listOf(feeHistoryRewardPercentile)),
        )
        .sendAsync(),
    )
      .thenAnswer {
        val feeHistoryResponse = EthFeeHistoryBlobExtended()
        val feeHistory = EthFeeHistoryBlobExtended.FeeHistoryBlobExtended(
          oldestBlock = "0x16",
          reward = (1000 until 1010).map { listOf(it.toString()) },
          baseFeePerGas = (10000 until 10011).map { it.toString() },
          gasUsedRatio = (10 until 20).map { it / 100.0 },
          baseFeePerBlobGas = (10000 until 10011).map { it.toString() },
          blobGasUsedRatio = (10 until 20).map { it / 100.0 },
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
          eq(feeHistoryBlockCount.toInt()),
          eq(DefaultBlockParameterName.LATEST),
          eq(listOf(feeHistoryRewardPercentile)),
        )
        .sendAsync(),
    )
      .thenAnswer {
        val feeHistoryResponse = EthFeeHistoryBlobExtended()
        val feeHistory = EthFeeHistoryBlobExtended.FeeHistoryBlobExtended(
          oldestBlock = "0x16",
          reward = (1000 until 1010).map { listOf(it.toString()) },
          baseFeePerGas = (10000 until 10011).map { it.toString() },
          gasUsedRatio = (10 until 20).map { it / 100.0 },
          baseFeePerBlobGas = emptyList(),
          blobGasUsedRatio = emptyList(),
        )
        feeHistoryResponse.result = feeHistory
        SafeFuture.completedFuture(feeHistoryResponse)
      }

    return web3jService
  }
}
