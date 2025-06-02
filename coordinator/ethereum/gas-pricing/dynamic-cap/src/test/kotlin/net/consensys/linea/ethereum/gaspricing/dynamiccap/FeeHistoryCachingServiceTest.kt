package net.consensys.linea.ethereum.gaspricing.dynamiccap

import io.vertx.core.Vertx
import io.vertx.junit5.Timeout
import io.vertx.junit5.VertxExtension
import io.vertx.junit5.VertxTestContext
import linea.domain.FeeHistory
import org.awaitility.Awaitility.await
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.TestInstance
import org.junit.jupiter.api.extension.ExtendWith
import org.mockito.Mockito.RETURNS_DEEP_STUBS
import org.mockito.Mockito.atLeast
import org.mockito.kotlin.any
import org.mockito.kotlin.atLeastOnce
import org.mockito.kotlin.doReturn
import org.mockito.kotlin.mock
import org.mockito.kotlin.verify
import org.mockito.kotlin.whenever
import org.web3j.protocol.Web3j
import org.web3j.protocol.core.methods.response.EthBlockNumber
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.math.BigInteger
import java.util.concurrent.CompletableFuture
import java.util.concurrent.TimeUnit
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.toJavaDuration

private val OneMWei = 1000000uL

@ExtendWith(VertxExtension::class)
@TestInstance(TestInstance.Lifecycle.PER_CLASS)
class FeeHistoryCachingServiceTest {
  private val feeHistory = FeeHistory(
    oldestBlock = 96uL,
    baseFeePerGas = listOf(1000, 1100, 1200, 1300, 1400, 1500).map { it.toULong().times(OneMWei) },
    reward = listOf(2000, 2100, 2200, 2300, 2400).map { listOf(it.toULong().times(OneMWei)) },
    gasUsedRatio = listOf(0.2, 0.4, 0.6, 0.8, 1.0).map { it },
    baseFeePerBlobGas = listOf(100, 110, 120, 130, 140, 150).map { it.toULong().times(OneMWei) },
    blobGasUsedRatio = listOf(0.5, 0.333, 0.167, 1.0, 0.667).map { it },
  )
  private val highestStoredL1BlockNumber = 100L
  private val latestL1BlockNumber = 200L
  private val pollingInterval = 10.milliseconds
  private val feeHistoryMaxBlockCount = 100U
  private val gasFeePercentile = 10.0
  private val feeHistoryStoragePeriodInBlocks = 120U
  private val feeHistoryWindowInBlocks = 100U
  private val numOfBlocksBeforeLatest = 0U
  private val deletedFeeHistoriesNum = 2
  private val storedFeeHistoriesNum = 100
  private lateinit var mockedL1Web3jClient: Web3j
  private lateinit var mockedL1FeeHistoryFetcher: GasPriceCapFeeHistoryFetcher
  private lateinit var mockedL1FeeHistoriesRepository: FeeHistoriesRepositoryWithCache

  private fun createFeeHistoryCachingService(
    vertx: Vertx,
    mockedL1FeeHistoriesRepository: FeeHistoriesRepositoryWithCache,
  ): FeeHistoryCachingService {
    return FeeHistoryCachingService(
      config = FeeHistoryCachingService.Config(
        pollingInterval = pollingInterval,
        feeHistoryMaxBlockCount = feeHistoryMaxBlockCount,
        gasFeePercentile = gasFeePercentile,
        feeHistoryStoragePeriodInBlocks = feeHistoryStoragePeriodInBlocks,
        feeHistoryWindowInBlocks = feeHistoryWindowInBlocks,
        numOfBlocksBeforeLatest = numOfBlocksBeforeLatest,
      ),
      vertx = vertx,
      web3jClient = mockedL1Web3jClient,
      feeHistoryFetcher = mockedL1FeeHistoryFetcher,
      feeHistoriesRepository = mockedL1FeeHistoriesRepository,
    )
  }

  @BeforeEach
  fun beforeEach() {
    mockedL1Web3jClient = mock<Web3j>(defaultAnswer = RETURNS_DEEP_STUBS)
    val mockBlockNumberReturn = mock<EthBlockNumber>()
    whenever(mockBlockNumberReturn.blockNumber).thenReturn(BigInteger.valueOf(latestL1BlockNumber))
    whenever(mockedL1Web3jClient.ethBlockNumber().sendAsync())
      .thenReturn(CompletableFuture.completedFuture(mockBlockNumberReturn))

    mockedL1FeeHistoryFetcher = mock<GasPriceCapFeeHistoryFetcher>(defaultAnswer = RETURNS_DEEP_STUBS) {
      on { getEthFeeHistoryData(any(), any()) } doReturn SafeFuture.completedFuture(feeHistory)
    }

    mockedL1FeeHistoriesRepository = mock<FeeHistoriesRepositoryWithCache> {
      on { findHighestBlockNumberWithPercentile(any()) } doReturn SafeFuture.completedFuture(highestStoredL1BlockNumber)
      on { saveNewFeeHistory(any()) } doReturn SafeFuture.completedFuture(Unit)
      on { cachePercentileGasFees(any(), any()) } doReturn SafeFuture.completedFuture(Unit)
      on { deleteFeeHistoriesUpToBlockNumber(any()) } doReturn SafeFuture.completedFuture(deletedFeeHistoriesNum)
      on { cacheNumOfFeeHistoriesFromBlockNumber(any(), any()) } doReturn SafeFuture.completedFuture(
        storedFeeHistoriesNum,
      )
    }
  }

  @Test
  @Timeout(5, timeUnit = TimeUnit.SECONDS)
  fun start_fetchAndSaveFeeHistories(vertx: Vertx, testContext: VertxTestContext) {
    createFeeHistoryCachingService(vertx, mockedL1FeeHistoriesRepository).start()
      .thenApply {
        await()
          .pollInterval(50.milliseconds.toJavaDuration())
          .untilAsserted {
            verify(mockedL1Web3jClient, atLeast(2)).ethBlockNumber()
            verify(mockedL1FeeHistoriesRepository, atLeastOnce()).findHighestBlockNumberWithPercentile(
              gasFeePercentile,
            )
            verify(mockedL1FeeHistoryFetcher, atLeast(2)).getEthFeeHistoryData(
              latestL1BlockNumber - feeHistoryWindowInBlocks.toLong() + 1L,
              latestL1BlockNumber - feeHistoryWindowInBlocks.toLong() + feeHistoryMaxBlockCount.toLong(),
            )
            verify(mockedL1FeeHistoriesRepository, atLeast(2)).saveNewFeeHistory(
              feeHistory,
            )
            verify(mockedL1FeeHistoriesRepository, atLeast(2)).cachePercentileGasFees(
              gasFeePercentile,
              latestL1BlockNumber - feeHistoryWindowInBlocks.toLong() + 1L,
            )
            verify(mockedL1FeeHistoriesRepository, atLeast(2))
              .deleteFeeHistoriesUpToBlockNumber(
                latestL1BlockNumber - feeHistoryStoragePeriodInBlocks.toLong(),
              )
            verify(mockedL1FeeHistoriesRepository, atLeast(2))
              .cacheNumOfFeeHistoriesFromBlockNumber(
                gasFeePercentile,
                latestL1BlockNumber - feeHistoryWindowInBlocks.toLong() + 1L,
              )
          }
        testContext.completeNow()
      }.whenException { th -> testContext.failNow(th) }
  }

  @Test
  @Timeout(5, timeUnit = TimeUnit.SECONDS)
  fun start_alwaysDeleteFeeHistoriesWhenError(vertx: Vertx, testContext: VertxTestContext) {
    mockedL1FeeHistoriesRepository = mock<FeeHistoriesRepositoryWithCache> {
      on { findHighestBlockNumberWithPercentile(any()) } doReturn SafeFuture.failedFuture(
        Error("Throw error for testing"),
      )
      on { deleteFeeHistoriesUpToBlockNumber(any()) } doReturn SafeFuture.completedFuture(deletedFeeHistoriesNum)
      on { cacheNumOfFeeHistoriesFromBlockNumber(any(), any()) } doReturn SafeFuture.completedFuture(
        storedFeeHistoriesNum,
      )
    }
    createFeeHistoryCachingService(vertx, mockedL1FeeHistoriesRepository).start()
      .thenApply {
        await()
          .pollInterval(50.milliseconds.toJavaDuration())
          .untilAsserted {
            verify(mockedL1Web3jClient, atLeast(2)).ethBlockNumber()
            verify(mockedL1FeeHistoriesRepository, atLeast(2))
              .deleteFeeHistoriesUpToBlockNumber(
                latestL1BlockNumber - feeHistoryStoragePeriodInBlocks.toLong(),
              )
            verify(mockedL1FeeHistoriesRepository, atLeast(2))
              .cacheNumOfFeeHistoriesFromBlockNumber(
                gasFeePercentile,
                latestL1BlockNumber - feeHistoryWindowInBlocks.toLong() + 1L,
              )
          }
        testContext.completeNow()
      }.whenException { th -> testContext.failNow(th) }
  }
}
