package net.consensys.zkevm.ethereum.finalization

import io.vertx.core.Vertx
import io.vertx.junit5.Timeout
import io.vertx.junit5.VertxExtension
import io.vertx.junit5.VertxTestContext
import linea.contract.l1.LineaContractVersion
import net.consensys.FakeFixedClock
import net.consensys.linea.ethereum.gaspricing.FakeGasPriceCapProvider
import net.consensys.linea.testing.submission.loadBlobsAndAggregations
import net.consensys.zkevm.coordinator.clients.smartcontract.LineaRollupSmartContractClient
import net.consensys.zkevm.domain.Aggregation
import net.consensys.zkevm.domain.BlobRecord
import net.consensys.zkevm.domain.BlobSubmittedEvent
import net.consensys.zkevm.domain.FinalizationSubmittedEvent
import net.consensys.zkevm.ethereum.Account
import net.consensys.zkevm.ethereum.ContractsManager
import net.consensys.zkevm.ethereum.MakeFileDelegatedContractsManager
import net.consensys.zkevm.ethereum.coordination.EventDispatcher
import net.consensys.zkevm.ethereum.submission.BlobSubmissionCoordinator
import net.consensys.zkevm.ethereum.submission.L1ShnarfBasedAlreadySubmittedBlobsFilter
import net.consensys.zkevm.persistence.AggregationsRepository
import net.consensys.zkevm.persistence.BlobsRepository
import net.consensys.zkevm.persistence.dao.aggregation.AggregationsRepositoryImpl
import net.consensys.zkevm.persistence.dao.aggregation.PostgresAggregationsDao
import net.consensys.zkevm.persistence.dao.blob.BlobsPostgresDao
import net.consensys.zkevm.persistence.dao.blob.BlobsRepositoryImpl
import net.consensys.zkevm.persistence.db.DbHelper
import net.consensys.zkevm.persistence.db.test.CleanDbTestSuiteParallel
import org.assertj.core.api.Assertions.assertThat
import org.awaitility.Awaitility.waitAtMost
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.TimeUnit
import java.util.function.Consumer
import kotlin.time.Duration.Companion.minutes
import kotlin.time.Duration.Companion.seconds
import kotlin.time.toJavaDuration

@ExtendWith(VertxExtension::class)
class BlobAndAggregationFinalizationIntTest : CleanDbTestSuiteParallel() {
  init {
    target = "4"
  }

  override val databaseName = DbHelper.generateUniqueDbName("coordinator-tests-submission-int-test")
  private val fakeClock = FakeFixedClock()
  private lateinit var lineaRollupContractForAggregationSubmission: LineaRollupSmartContractClient
  private lateinit var contractDeploymentAccount: Account
  private lateinit var aggregationsRepository: AggregationsRepository
  private lateinit var blobsRepository: BlobsRepository
  private lateinit var blobSubmissionCoordinator: BlobSubmissionCoordinator
  private lateinit var aggregationFinalizationCoordinator: AggregationFinalizationCoordinator
  private val testDataDir = "testdata/coordinator/prover/v2/"
  private lateinit var blobSubmittedEvent: BlobSubmittedEvent
  private var blobSubmissionDelay = 0L
  private lateinit var finalizationSubmittedEvent: FinalizationSubmittedEvent
  private var finalizationSubmissionDelay = 0L
  private var acceptedBlob = 0UL

  // 1-block-per-blob test data has 3 aggregations: 1..7, 8..14, 15..21.
  // We will upgrade the contract in the middle of 2nd aggregation: 12
  // shall submit blob 12, stop submission, upgrade the contract and resume with blob 13
  // val lastSubmittedBlobs = blobs.filter { it.startBlockNumber == 7UL }
  private lateinit var aggregations: List<Aggregation>
  private lateinit var blobs: List<BlobRecord>

  private fun setupTest(
    vertx: Vertx,
    smartContractVersion: LineaContractVersion
  ) {
    if (listOf(LineaContractVersion.V6).contains(smartContractVersion).not()) {
      // V6 with prover V3 is soon comming, so we will need to update/extend this test setup
      throw IllegalArgumentException("unsupported contract version=$smartContractVersion!")
    }
    val rollupDeploymentFuture = ContractsManager.get()
      .deployLineaRollup(numberOfOperators = 2, contractVersion = smartContractVersion)
    // load files from FS while smc deploy
    loadBlobsAndAggregations(
      blobsResponsesDir = "$testDataDir/compression/responses",
      aggregationsResponsesDir = "$testDataDir/aggregation/responses"
    )
      .let { (blobs, aggregations) ->
        this.blobs = blobs
        this.aggregations = aggregations
      }
    // wait smc deployment finishes
    val rollupDeploymentResult = rollupDeploymentFuture.get()

    contractDeploymentAccount = rollupDeploymentResult.contractDeploymentAccount

    blobsRepository = BlobsRepositoryImpl(
      BlobsPostgresDao(
        config = BlobsPostgresDao.Config(maxBlobsToReturn = 100U),
        connection = sqlClient,
        clock = fakeClock
      )
    )
    aggregationsRepository = AggregationsRepositoryImpl(PostgresAggregationsDao(sqlClient, fakeClock))

    val lineaRollupContractForDataSubmissionV6 = rollupDeploymentResult.rollupOperatorClient

    val acceptedBlobEndBlockNumberConsumer = Consumer<ULong> { acceptedBlob = it }

    @Suppress("DEPRECATION")
    val alreadySubmittedBlobFilter = L1ShnarfBasedAlreadySubmittedBlobsFilter(
      lineaRollup = lineaRollupContractForDataSubmissionV6,
      acceptedBlobEndBlockNumberConsumer = acceptedBlobEndBlockNumberConsumer
    )
    val blobSubmittedEventConsumers = mapOf(
      Consumer<BlobSubmittedEvent> { blobSubmittedEvent = it } to "Blob Submitted Consumer 1",
      Consumer<BlobSubmittedEvent> { blobSubmissionDelay = it.getSubmissionDelay() } to "Blob Submitted Consumer 2"
    )

    blobSubmissionCoordinator = run {
      BlobSubmissionCoordinator.create(
        config = BlobSubmissionCoordinator.Config(
          pollingInterval = 6.seconds,
          proofSubmissionDelay = 0.seconds,
          maxBlobsToSubmitPerTick = 100u,
          targetBlobsToSubmitPerTx = 6u
        ),
        blobsRepository = blobsRepository,
        aggregationsRepository = aggregationsRepository,
        lineaSmartContractClient = lineaRollupContractForDataSubmissionV6,
        alreadySubmittedBlobsFilter = alreadySubmittedBlobFilter,
        gasPriceCapProvider = FakeGasPriceCapProvider(),
        blobSubmittedEventDispatcher = EventDispatcher(blobSubmittedEventConsumers),
        vertx = vertx,
        clock = fakeClock
      )
    }

    aggregationFinalizationCoordinator = run {
      lineaRollupContractForAggregationSubmission = MakeFileDelegatedContractsManager
        .connectToLineaRollupContract(
          rollupDeploymentResult.contractAddress,
          rollupDeploymentResult.rollupOperators[1].txManager

        )

      val submittedFinalizationConsumers = mapOf(
        Consumer<FinalizationSubmittedEvent> { finalizationSubmittedEvent = it } to "Finalization Submitted Consumer 1",
        Consumer<FinalizationSubmittedEvent> { finalizationSubmissionDelay = it.getSubmissionDelay() }
          to "Finalization Submitted Consumer 2"
      )

      val aggregationSubmitter = AggregationSubmitterImpl(
        lineaRollup = lineaRollupContractForAggregationSubmission,
        gasPriceCapProvider = FakeGasPriceCapProvider(),
        aggregationSubmittedEventConsumer = EventDispatcher(submittedFinalizationConsumers),
        clock = fakeClock
      )

      AggregationFinalizationCoordinator(
        config = AggregationFinalizationCoordinator.Config(
          pollingInterval = 6.seconds,
          proofSubmissionDelay = 0.seconds
        ),
        aggregationSubmitter = aggregationSubmitter,
        aggregationsRepository = aggregationsRepository,
        blobsRepository = blobsRepository,
        lineaRollup = lineaRollupContractForAggregationSubmission,
        alreadySubmittedBlobsFilter = alreadySubmittedBlobFilter,
        vertx = vertx,
        clock = fakeClock
      )
    }
  }

  private fun testSubmission(
    vertx: Vertx,
    testContext: VertxTestContext,
    smartContractVersion: LineaContractVersion
  ) {
    setupTest(vertx, smartContractVersion)

    SafeFuture.allOf(
      SafeFuture.collectAll(blobs.map { blobsRepository.saveNewBlob(it) }.stream()),
      SafeFuture.collectAll(aggregations.map { aggregationsRepository.saveNewAggregation(it) }.stream())
    ).get()

    val aggEndTime = aggregations.last().aggregationProof!!.finalTimestamp
    val blobsEndTime = blobs.last().endBlockTime
    val endTime = if (aggEndTime > blobsEndTime) aggEndTime else blobsEndTime

    // submission do the cutoff by minutes, so we need to add 1 minute to the end time
    fakeClock.setTimeTo(endTime.plus(1.minutes))

    blobSubmissionCoordinator.start()
    aggregationFinalizationCoordinator.start()
      .thenApply {
        waitAtMost(2.minutes.toJavaDuration())
          .pollInterval(1.seconds.toJavaDuration())
          .untilAsserted {
            val finalizedBlockNumber = lineaRollupContractForAggregationSubmission.finalizedL2BlockNumber().get()
            assertThat(finalizedBlockNumber).isEqualTo(aggregations.last().endBlockNumber)
            assertThat(blobSubmittedEvent.blobs.last().endBlockNumber).isEqualTo(blobs[20].endBlockNumber)
            assertThat(acceptedBlob).isEqualTo(blobs[20].endBlockNumber)
            assertThat(finalizationSubmittedEvent.endBlockNumber).isEqualTo(
              aggregations.last().endBlockNumber
            )
            assertThat(blobSubmissionDelay).isGreaterThan(0L)
            assertThat(finalizationSubmissionDelay).isGreaterThan(0L)
          }
        testContext.completeNow()
      }.whenException(testContext::failNow)
  }

  @Test
  @Timeout(3, timeUnit = TimeUnit.MINUTES)
  fun `submission works with contract V6`(
    vertx: Vertx,
    testContext: VertxTestContext
  ) {
    testSubmission(vertx, testContext, LineaContractVersion.V6)
  }
}
