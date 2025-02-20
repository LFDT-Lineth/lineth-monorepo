package net.consensys.zkevm.persistence.dao.blob

import io.vertx.core.Vertx
import io.vertx.junit5.VertxExtension
import linea.domain.BlockIntervals
import linea.kotlin.setFirstByteToZero
import linea.kotlin.trimToSecondPrecision
import net.consensys.FakeFixedClock
import net.consensys.zkevm.coordinator.clients.BlobCompressionProof
import net.consensys.zkevm.domain.createBlobRecord
import net.consensys.zkevm.persistence.db.PersistenceRetryer
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import org.mockito.Mockito.mock
import org.mockito.Mockito.times
import org.mockito.kotlin.any
import org.mockito.kotlin.eq
import org.mockito.kotlin.verify
import org.mockito.kotlin.whenever
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.random.Random
import kotlin.time.Duration.Companion.milliseconds

@ExtendWith(VertxExtension::class)
class RetryingBlobsPostgresDaoTest {
  private lateinit var retryingBatchesPostgresDao: RetryingBlobsPostgresDao
  private val delegateBlobsDao = mock<BlobsPostgresDao>()

  private val fakeClock = FakeFixedClock()
  private val now = fakeClock.now().trimToSecondPrecision()
  private val blobRecord = createBlobRecord(
    startBlockNumber = 0U,
    endBlockNumber = 10U,
    startBlockTime = now
  )
  private val blobCompressionProof = BlobCompressionProof(
    compressedData = Random.nextBytes(32).setFirstByteToZero(),
    conflationOrder = BlockIntervals(41U, listOf(60U)),
    prevShnarf = Random.nextBytes(32),
    parentStateRootHash = Random.nextBytes(32).setFirstByteToZero(),
    finalStateRootHash = Random.nextBytes(32).setFirstByteToZero(),
    parentDataHash = Random.nextBytes(32).setFirstByteToZero(),
    dataHash = Random.nextBytes(32).setFirstByteToZero(),
    snarkHash = Random.nextBytes(32),
    expectedX = Random.nextBytes(32),
    expectedY = Random.nextBytes(32),
    expectedShnarf = Random.nextBytes(32).setFirstByteToZero(),
    decompressionProof = Random.nextBytes(512),
    proverVersion = "mock-0.0.0",
    verifierID = 6789,
    commitment = ByteArray(0),
    kzgProofContract = ByteArray(0),
    kzgProofSidecar = ByteArray(0)
  )

  @BeforeEach
  fun beforeEach(vertx: Vertx) {
    retryingBatchesPostgresDao = RetryingBlobsPostgresDao(
      delegate = delegateBlobsDao,
      PersistenceRetryer(
        vertx = vertx,
        PersistenceRetryer.Config(
          backoffDelay = 1.milliseconds
        )
      )
    )

    whenever(delegateBlobsDao.saveNewBlob(eq(blobRecord)))
      .thenReturn(SafeFuture.completedFuture(Unit))

    whenever(delegateBlobsDao.getConsecutiveBlobsFromBlockNumber(any(), eq(now)))
      .thenReturn(SafeFuture.completedFuture(listOf(blobRecord)))

    whenever(delegateBlobsDao.findBlobByStartBlockNumber(any()))
      .thenReturn(SafeFuture.completedFuture(blobRecord))

    whenever(delegateBlobsDao.findBlobByEndBlockNumber(any()))
      .thenReturn(SafeFuture.completedFuture(blobRecord))

    whenever(delegateBlobsDao.deleteBlobsUpToEndBlockNumber(any()))
      .thenReturn(SafeFuture.completedFuture(1))

    whenever(delegateBlobsDao.deleteBlobsAfterBlockNumber(any()))
      .thenReturn(SafeFuture.completedFuture(1))
  }

  @Test
  fun `retrying batches dao should delegate all queries to standard dao`() {
    retryingBatchesPostgresDao.saveNewBlob(blobRecord)
    verify(delegateBlobsDao, times(1)).saveNewBlob(eq(blobRecord))

    retryingBatchesPostgresDao.getConsecutiveBlobsFromBlockNumber(0U, now)
    verify(delegateBlobsDao, times(1)).getConsecutiveBlobsFromBlockNumber(any(), eq(now))

    retryingBatchesPostgresDao.findBlobByStartBlockNumber(0U)
    verify(delegateBlobsDao, times(1)).findBlobByStartBlockNumber(any())

    retryingBatchesPostgresDao.findBlobByEndBlockNumber(10U)
    verify(delegateBlobsDao, times(1)).findBlobByEndBlockNumber(any())

    retryingBatchesPostgresDao.deleteBlobsUpToEndBlockNumber(10U)
    verify(delegateBlobsDao, times(1)).deleteBlobsUpToEndBlockNumber(any())

    retryingBatchesPostgresDao.deleteBlobsAfterBlockNumber(0U)
    verify(delegateBlobsDao, times(1)).deleteBlobsAfterBlockNumber(any())
  }
}
