package net.consensys.zkevm.ethereum.coordination.blob

import build.linea.clients.GetZkEVMStateMerkleProofResponse
import build.linea.clients.StateManagerClientV1
import com.fasterxml.jackson.databind.node.ArrayNode
import com.fasterxml.jackson.module.kotlin.jacksonObjectMapper
import com.github.michaelbull.result.Ok
import io.vertx.core.Vertx
import io.vertx.junit5.VertxExtension
import io.vertx.junit5.VertxTestContext
import kotlinx.datetime.Clock
import kotlinx.datetime.Instant
import linea.domain.BlockIntervals
import linea.kotlin.ByteArrayExt
import net.consensys.linea.traces.TracesCountersV2
import net.consensys.zkevm.coordinator.clients.BlobCompressionProof
import net.consensys.zkevm.coordinator.clients.BlobCompressionProofRequest
import net.consensys.zkevm.coordinator.clients.BlobCompressionProverClientV2
import net.consensys.zkevm.domain.Blob
import net.consensys.zkevm.domain.ConflationCalculationResult
import net.consensys.zkevm.domain.ConflationTrigger
import net.consensys.zkevm.domain.ProofIndex
import net.consensys.zkevm.domain.createBlobRecord
import net.consensys.zkevm.persistence.BlobsRepository
import net.consensys.zkevm.persistence.dao.blob.BlobsPostgresDao
import net.consensys.zkevm.persistence.dao.blob.BlobsRepositoryImpl
import net.consensys.zkevm.persistence.db.DbHelper
import net.consensys.zkevm.persistence.db.test.CleanDbTestSuiteParallel
import org.apache.tuweni.bytes.Bytes32
import org.assertj.core.api.Assertions.assertThat
import org.awaitility.Awaitility.waitAtMost
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import org.mockito.Mockito
import org.mockito.kotlin.any
import org.mockito.kotlin.eq
import org.mockito.kotlin.mock
import org.mockito.kotlin.spy
import org.mockito.kotlin.times
import org.mockito.kotlin.verify
import org.mockito.kotlin.whenever
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.nio.file.Path
import kotlin.random.Random
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds
import kotlin.time.toJavaDuration

@ExtendWith(VertxExtension::class)
class BlobCompressionProofCoordinatorIntTest : CleanDbTestSuiteParallel() {
  init {
    target = "4"
  }

  override val databaseName = DbHelper.generateUniqueDbName(
    "blob-compression-proof-coordinator",
  )
  private val maxBlobsToReturn = 30u
  private var timeToReturn: Instant = Clock.System.now()
  private val fixedClock =
    object : Clock {
      override fun now(): Instant {
        return timeToReturn
      }
    }
  private lateinit var blobsPostgresDao: BlobsPostgresDao
  private lateinit var blobCompressionProofCoordinator: BlobCompressionProofCoordinator

  private val expectedStartBlock = 1UL
  private val expectedEndBlock = 100UL
  private val blobHandlerPollingInterval = 50.milliseconds
  private val expectedStartBlockTime = Instant.fromEpochMilliseconds(fixedClock.now().toEpochMilliseconds())
  private val expectedEndBlockTime = Instant.fromEpochMilliseconds(
    fixedClock.now().plus(1200.seconds).toEpochMilliseconds(),
  )
  private var expectedBlobCompressionProofResponse: BlobCompressionProof? = null
  private val proofIndexExpectedProofResponseMap: MutableMap<ProofIndex, BlobCompressionProof> = mutableMapOf()

  private val zkStateClientMock = mock<StateManagerClientV1>()
  private val blobCompressionProverClientMock = mock<BlobCompressionProverClientV2>()
  private val blobZkStateProvider = mock<BlobZkStateProvider>()
  private lateinit var mockShnarfCalculator: BlobShnarfCalculator
  private lateinit var blobsRepositorySpy: BlobsRepository

  private val testFilePath = "../../../testdata/type2state-manager/state-proof.json"
  private val json = jacksonObjectMapper().readTree(Path.of(testFilePath).toFile())
  private val zkStateManagerVersion = json.get("zkStateManagerVersion").asText()
  private val zkStateMerkleProof = json.get("zkStateMerkleProof") as ArrayNode

  @BeforeEach
  fun beforeEach(vertx: Vertx) {
    proofIndexExpectedProofResponseMap.clear()
    blobsPostgresDao =
      BlobsPostgresDao(
        config = BlobsPostgresDao.Config(
          maxBlobsToReturn,
        ),
        connection = sqlClient,
        clock = fixedClock,
      )
    whenever(zkStateClientMock.rollupGetStateMerkleProof(any()))
      .thenAnswer {
        SafeFuture.completedFuture(
          Ok(
            GetZkEVMStateMerkleProofResponse(
              zkStateManagerVersion = zkStateManagerVersion,
              zkStateMerkleProof = zkStateMerkleProof,
              zkParentStateRootHash = ByteArrayExt.random32(),
              zkEndStateRootHash = ByteArrayExt.random32(),
            ),
          ),
        )
      }
    whenever(blobZkStateProvider.getBlobZKState(any()))
      .thenReturn(
        SafeFuture.completedFuture(
          BlobZkState(
            parentStateRootHash = Bytes32.random().toArray(),
            finalStateRootHash = Bytes32.random().toArray(),
          ),
        ),
      )
    whenever(blobCompressionProverClientMock.createProofRequest(any()))
      .thenAnswer { invocationMock ->
        val proofReq = invocationMock.arguments[0] as BlobCompressionProofRequest
        expectedBlobCompressionProofResponse = BlobCompressionProof(
          compressedData = proofReq.compressedData,
          conflationOrder = BlockIntervals(
            startingBlockNumber = proofReq.startBlockNumber,
            upperBoundaries = proofReq.conflations.map { it.endBlockNumber },
          ),
          prevShnarf = proofReq.prevShnarf,
          parentStateRootHash = proofReq.parentStateRootHash,
          finalStateRootHash = proofReq.finalStateRootHash,
          parentDataHash = proofReq.parentDataHash,
          dataHash = proofReq.expectedShnarfResult.dataHash,
          snarkHash = proofReq.expectedShnarfResult.snarkHash,
          expectedX = proofReq.expectedShnarfResult.expectedX,
          expectedY = proofReq.expectedShnarfResult.expectedY,
          expectedShnarf = proofReq.expectedShnarfResult.expectedShnarf,
          decompressionProof = Random.nextBytes(512),
          proverVersion = "mock-0.0.0",
          verifierID = 6789,
          commitment = Random.nextBytes(48),
          kzgProofContract = Random.nextBytes(48),
          kzgProofSidecar = Random.nextBytes(48),
        )
        val proofIndex = ProofIndex(
          startBlockNumber = proofReq.startBlockNumber,
          endBlockNumber = proofReq.conflations.last().endBlockNumber,
          hash = proofReq.expectedShnarfResult.dataHash,
        )
        proofIndexExpectedProofResponseMap[proofIndex] = expectedBlobCompressionProofResponse!!
        SafeFuture.completedFuture(proofIndex)
      }

    whenever(blobCompressionProverClientMock.findProofResponse(any()))
      .thenAnswer { invocationMock ->
        val proofIndex = invocationMock.arguments[0] as ProofIndex
        SafeFuture.completedFuture(proofIndexExpectedProofResponseMap[proofIndex])
      }

    mockShnarfCalculator = spy(FakeBlobShnarfCalculator())
    blobsRepositorySpy = spy(
      BlobsRepositoryImpl(
        blobsPostgresDao,
      ),
    )

    val rollingBlobShnarfCalculator = RollingBlobShnarfCalculator(
      blobShnarfCalculator = mockShnarfCalculator,
      blobsRepository = blobsRepositorySpy,
      genesisShnarf = ByteArray(32),
    )

    blobCompressionProofCoordinator = BlobCompressionProofCoordinator(
      vertx = vertx,
      blobCompressionProverClient = blobCompressionProverClientMock,
      rollingBlobShnarfCalculator = rollingBlobShnarfCalculator,
      blobZkStateProvider = blobZkStateProvider,
      config = BlobCompressionProofCoordinator.Config(
        pollingInterval = blobHandlerPollingInterval,
      ),
      blobCompressionProofHandler = { blobRecord -> blobsRepositorySpy.saveNewBlob(blobRecord) },
      metricsFacade = mock(defaultAnswer = Mockito.RETURNS_DEEP_STUBS),
    )
    blobCompressionProofCoordinator.start()
  }

  private fun createConsecutiveBlobs(
    numberOfBlobs: Int,
    conflationStep: ULong = 10UL,
    startBlockNumber: ULong,
    startBlockTime: Instant,
  ): List<Blob> {
    var currentBlockNumber = startBlockNumber
    var currentBlockTime = startBlockTime
    return (1..numberOfBlobs).map {
      val endBlockNumber = currentBlockNumber + conflationStep
      val endBlockTime = currentBlockTime.plus((12 * conflationStep.toInt()).seconds)
      val blob = Blob(
        conflations = listOf(
          ConflationCalculationResult(
            startBlockNumber = currentBlockNumber,
            endBlockNumber = endBlockNumber,
            conflationTrigger = ConflationTrigger.TRACES_LIMIT,
            tracesCounters = TracesCountersV2.EMPTY_TRACES_COUNT,
          ),
        ),
        compressedData = Random.nextBytes(128),
        startBlockTime = currentBlockTime,
        endBlockTime = endBlockTime,
      )
      currentBlockNumber = endBlockNumber + 1UL
      currentBlockTime = endBlockTime.plus(12.seconds)
      blob
    }
  }

  @Test
  fun `handle blob event and update blob record with blob compression proof`(testContext: VertxTestContext) {
    val prevBlobRecord = createBlobRecord(
      startBlockNumber = expectedStartBlock,
      endBlockNumber = expectedEndBlock,
      startBlockTime = expectedStartBlockTime,
    )
    timeToReturn = Clock.System.now()
    blobsPostgresDao.saveNewBlob(prevBlobRecord).get()

    val blobEventStartBlock = expectedEndBlock + 1UL
    val blobEventEndBlock = expectedEndBlock + 100UL
    val blobEvent = Blob(
      conflations = listOf(
        ConflationCalculationResult(
          startBlockNumber = blobEventStartBlock,
          endBlockNumber = blobEventEndBlock,
          conflationTrigger = ConflationTrigger.TRACES_LIMIT,
          tracesCounters = TracesCountersV2.EMPTY_TRACES_COUNT,
        ),
        ConflationCalculationResult(
          startBlockNumber = blobEventEndBlock + 1UL,
          endBlockNumber = blobEventEndBlock + 200UL,
          conflationTrigger = ConflationTrigger.TRACES_LIMIT,
          tracesCounters = TracesCountersV2.EMPTY_TRACES_COUNT,
        ),
        ConflationCalculationResult(
          startBlockNumber = blobEventEndBlock + 201UL,
          endBlockNumber = blobEventEndBlock + 300UL,
          conflationTrigger = ConflationTrigger.TRACES_LIMIT,
          tracesCounters = TracesCountersV2.EMPTY_TRACES_COUNT,
        ),
      ),
      compressedData = Random.nextBytes(128),
      startBlockTime = prevBlobRecord.endBlockTime.plus(12.seconds),
      endBlockTime = prevBlobRecord.endBlockTime.plus(3600.seconds),
    )

    timeToReturn = Clock.System.now()
    blobCompressionProofCoordinator.handleBlob(blobEvent).get()

    waitAtMost(10.seconds.toJavaDuration())
      .pollInterval(200.milliseconds.toJavaDuration())
      .untilAsserted {
        val actualBlobs = blobsPostgresDao.getConsecutiveBlobsFromBlockNumber(
          expectedStartBlock,
          blobEvent.endBlockTime.plus(1.seconds),
        ).get()

        assertThat(actualBlobs).size().isEqualTo(2)
        verify(blobsRepositorySpy, times(1)).findBlobByEndBlockNumber(any())
        verify(blobsRepositorySpy, times(1)).findBlobByEndBlockNumber(eq(expectedEndBlock.toLong()))
        val blobCompressionProof = actualBlobs[1].blobCompressionProof
        assertThat(blobCompressionProof).isEqualTo(expectedBlobCompressionProofResponse)
        assertThat(actualBlobs[1].startBlockNumber).isEqualTo(blobEventStartBlock)
        assertThat(actualBlobs[1].endBlockNumber).isEqualTo(blobEventEndBlock + 300UL)
        assertThat(actualBlobs[1].batchesCount).isEqualTo(3U)
        assertThat(actualBlobs[1].blobHash).isEqualTo(blobCompressionProof?.dataHash)
        assertThat(blobCompressionProof?.parentDataHash).isEqualTo(prevBlobRecord.blobHash)
        assertThat(blobCompressionProof?.prevShnarf).isEqualTo(prevBlobRecord.expectedShnarf)
        verify(mockShnarfCalculator).calculateShnarf(any(), any(), any(), any(), any())
        verify(blobCompressionProverClientMock).createProofRequest(any())
        verify(blobCompressionProverClientMock).findProofResponse(any())
      }
    testContext.completeNow()
  }

  @Test
  fun `handle blob event and update blob record with blob compression proof when prev blob record not found`(
    testContext: VertxTestContext,
  ) {
    val blobEventStartBlock = expectedEndBlock + 1UL
    val blobEventEndBlock = expectedEndBlock + 100UL
    val blobEvent = Blob(
      conflations = listOf(
        ConflationCalculationResult(
          startBlockNumber = blobEventStartBlock,
          endBlockNumber = blobEventEndBlock,
          conflationTrigger = ConflationTrigger.TRACES_LIMIT,
          tracesCounters = TracesCountersV2.EMPTY_TRACES_COUNT,
        ),
      ),
      compressedData = Random.nextBytes(128),
      startBlockTime = expectedEndBlockTime.plus(12.seconds),
      endBlockTime = expectedEndBlockTime.plus(1200.seconds),
    )

    timeToReturn = Clock.System.now()
    blobCompressionProofCoordinator.handleBlob(blobEvent).get()

    waitAtMost(10.seconds.toJavaDuration())
      .pollInterval(200.milliseconds.toJavaDuration())
      .untilAsserted {
        val actualBlobs = blobsPostgresDao.getConsecutiveBlobsFromBlockNumber(
          blobEventStartBlock,
          blobEvent.endBlockTime.plus(1.seconds),
        ).get()

        assertThat(actualBlobs).size().isEqualTo(0)
      }
    testContext.completeNow()
  }

  @Test
  fun `handle blob events and update blob record with blob compression proof with correct parent blob data`(
    testContext: VertxTestContext,
  ) {
    val prevBlobRecord = createBlobRecord(
      startBlockNumber = expectedStartBlock,
      endBlockNumber = expectedEndBlock,
      startBlockTime = expectedStartBlockTime,
    )
    timeToReturn = Clock.System.now()
    blobsPostgresDao.saveNewBlob(prevBlobRecord).get()

    val blobs = createConsecutiveBlobs(
      numberOfBlobs = maxBlobsToReturn.toInt() - 1,
      startBlockNumber = expectedEndBlock + 1UL,
      startBlockTime = prevBlobRecord.endBlockTime.plus(12.seconds),
    )

    timeToReturn = Clock.System.now()
    SafeFuture.allOf(
      blobs.map {
        blobCompressionProofCoordinator.handleBlob(it)
      }.stream(),
    ).get()

    waitAtMost(10.seconds.toJavaDuration())
      .pollInterval(200.milliseconds.toJavaDuration())
      .untilAsserted {
        val actualBlobs = blobsPostgresDao.getConsecutiveBlobsFromBlockNumber(
          expectedStartBlock,
          blobs.last().endBlockTime.plus(1.seconds),
        ).get()

        assertThat(actualBlobs).size().isEqualTo(blobs.size + 1)
        verify(blobsRepositorySpy, times(1)).findBlobByEndBlockNumber(any())
        verify(blobsRepositorySpy, times(1)).findBlobByEndBlockNumber(eq(expectedEndBlock.toLong()))

        var previousBlob = actualBlobs.first()
        actualBlobs.drop(1).forEach { blobRecord ->
          val blobCompressionProof = blobRecord.blobCompressionProof!!
          testContext.verify {
            assertThat(blobCompressionProof.parentDataHash).isEqualTo(previousBlob.blobHash)
            assertThat(blobCompressionProof.prevShnarf).isEqualTo(previousBlob.expectedShnarf)
          }
          previousBlob = blobRecord
        }
      }
    testContext.completeNow()
  }

  @Test
  fun `test blob handle failures re-queue's the blob`(testContext: VertxTestContext) {
    val prevBlobRecord = createBlobRecord(
      startBlockNumber = expectedStartBlock,
      endBlockNumber = expectedEndBlock,
      startBlockTime = expectedStartBlockTime,
    )
    timeToReturn = Clock.System.now()
    blobsPostgresDao.saveNewBlob(prevBlobRecord).get()

    val blobs = createConsecutiveBlobs(
      numberOfBlobs = maxBlobsToReturn.toInt() - 1,
      startBlockNumber = expectedEndBlock + 1UL,
      startBlockTime = prevBlobRecord.endBlockTime.plus(12.seconds),
    )
    val maxMockedBlobZkStateFailures = 10
    var blobZkStateFailures = 0
    var blobZkStateCount = 0

    Mockito.reset(blobZkStateProvider)
    whenever(blobZkStateProvider.getBlobZKState(any())).thenAnswer {
      blobZkStateCount += 1
      if (blobZkStateFailures <= maxMockedBlobZkStateFailures && blobZkStateCount % 2 == 0) {
        blobZkStateFailures += 1
        SafeFuture.failedFuture<BlobZkState>(RuntimeException("Forced mock blobZkStateProvider failure"))
      } else {
        SafeFuture.completedFuture(
          BlobZkState(
            parentStateRootHash = Bytes32.random().toArray(),
            finalStateRootHash = Bytes32.random().toArray(),
          ),
        )
      }
    }

    timeToReturn = Clock.System.now()
    SafeFuture.allOf(
      blobs.map {
        blobCompressionProofCoordinator.handleBlob(it)
      }.stream(),
    ).get()

    waitAtMost(100.seconds.toJavaDuration())
      .pollInterval(200.milliseconds.toJavaDuration())
      .untilAsserted {
        val actualBlobs = blobsPostgresDao.getConsecutiveBlobsFromBlockNumber(
          expectedStartBlock,
          blobs.last().endBlockTime.plus(1.seconds),
        ).get()

        assertThat(actualBlobs).size().isEqualTo(blobs.size + 1)
        verify(blobsRepositorySpy, times(1)).findBlobByEndBlockNumber(any())
        verify(blobsRepositorySpy, times(1)).findBlobByEndBlockNumber(eq(expectedEndBlock.toLong()))

        var previousBlob = actualBlobs.first()
        actualBlobs.drop(1).forEach { blobRecord ->
          val blobCompressionProof = blobRecord.blobCompressionProof!!
          testContext.verify {
            assertThat(blobCompressionProof.parentDataHash).isEqualTo(previousBlob.blobHash)
            assertThat(blobCompressionProof.prevShnarf).isEqualTo(previousBlob.expectedShnarf)
          }
          previousBlob = blobRecord
        }
      }
    testContext.completeNow()
  }
}
