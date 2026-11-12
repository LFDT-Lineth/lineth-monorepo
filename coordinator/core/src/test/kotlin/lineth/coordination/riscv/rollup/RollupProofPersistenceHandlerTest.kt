package lineth.coordination.riscv.rollup

import linea.clients.dummyRollupProofResponse
import linea.domain.BlobRecordV2
import linea.domain.BlockIntervalProofIndex
import linea.domain.createBlobRecordV2
import lineth.persistence.BlobsRepositoryV2
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Test
import org.mockito.kotlin.any
import org.mockito.kotlin.argumentCaptor
import org.mockito.kotlin.mock
import org.mockito.kotlin.never
import org.mockito.kotlin.verify
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.random.Random

class RollupProofPersistenceHandlerTest {
  private val blobsRepository = mock<BlobsRepositoryV2> {
    on { saveNewBlob(any()) }.thenReturn(SafeFuture.completedFuture(Unit))
  }
  private val handler = RollupProofPersistenceHandler(blobsRepository)

  private val record = createBlobRecordV2(
    startBlockNumber = 1UL,
    endBlockNumber = 10UL,
    parentDataTailTakeBytes = 100,
    finalDataTailDiscardBytes = 200,
  )
  private val proofIndex = BlockIntervalProofIndex(
    startBlockNumber = record.startBlockNumber,
    endBlockNumber = record.endBlockNumber,
    startBlockTimestamp = record.startBlockTimestamp,
    hash = record.proofHash,
  )
  private val context = RollupProofPoller.ProofContext(
    proofIndex = proofIndex,
    blobsData = record.blobsData,
    parentDataRollingHash = record.parentDataRollingHash,
    dataRollingHash = record.dataRollingHash,
    parentDataTailTakeBytes = record.parentDataTailTakeBytes,
    boundaryPrevDataRollingHash = record.boundaryPrevDataRollingHash,
    finalDataTailDiscardBytes = record.finalDataTailDiscardBytes,
    startBlockTimestamp = record.startBlockTimestamp,
    endBlockTimestamp = record.endBlockTimestamp,
    totalBatchesCount = record.totalBatchesCount.toInt(),
  )

  private fun proof(
    endDataRollingHash: ByteArray = record.dataRollingHash,
    finalDataTailDiscardBytes: Int = record.finalDataTailDiscardBytes,
  ) = dummyRollupProofResponse(proofIndex).let {
    it.copy(
      publicInputs = it.publicInputs.copy(
        parentDataRollingHash = record.parentDataRollingHash,
        endDataRollingHash = endDataRollingHash,
        parentDataTailTakeBytes = record.parentDataTailTakeBytes,
        finalDataTailDiscardBytes = finalDataTailDiscardBytes,
      ),
    )
  }

  @Test
  fun `a proof matching the request is persisted as the rollup row`() {
    handler.acceptNewRollupProof(proof(), context).get()

    val saved = argumentCaptor<BlobRecordV2>()
    verify(blobsRepository).saveNewBlob(saved.capture())
    assertThat(saved.firstValue).isEqualTo(record)
  }

  @Test
  fun `a proof whose DA outputs differ from the request is not persisted`() {
    assertThatThrownBy { handler.acceptNewRollupProof(proof(endDataRollingHash = Random.nextBytes(32)), context).get() }
      .hasMessageContaining("rollup proof DA outputs differ from the request")
    assertThatThrownBy { handler.acceptNewRollupProof(proof(finalDataTailDiscardBytes = 0), context).get() }
      .hasMessageContaining("finalDataTailDiscardBytes=0 expected=200")

    verify(blobsRepository, never()).saveNewBlob(any())
  }
}
