package lineth.coordination.riscv.rollup

import linea.clients.dummyRollupProofResponse
import linea.domain.BlockIntervalProofIndex
import linea.domain.RiscvRollupRecord
import linea.domain.createRiscvRollupRecord
import lineth.persistence.RiscvRollupsDao
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
  private val rollupsDao = mock<RiscvRollupsDao> {
    on { saveRollupWithBlobs(any()) }.thenReturn(SafeFuture.completedFuture(Unit))
  }
  private val handler = RollupProofPersistenceHandler(rollupsDao)

  private val record = createRiscvRollupRecord(
    startBlockNumber = 1UL,
    endBlockNumber = 10UL,
    parentSharedBlobCarryOverTailBytes = 100u,
    finalSharedBlobCarryOverTailBytes = 200u,
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
    parentSharedBlobCarryOverTailBytes = record.parentSharedBlobCarryOverTailBytes,
    parentBlobDataRollingHash = record.parentBlobDataRollingHash,
    finalSharedBlobCarryOverTailBytes = record.finalSharedBlobCarryOverTailBytes,
    startBlockTimestamp = record.startBlockTimestamp,
    endBlockTimestamp = record.endBlockTimestamp,
    totalBatchesCount = record.totalBatchesCount.toInt(),
  )

  private fun proof(
    endDataRollingHash: ByteArray = record.dataRollingHash,
    finalSharedBlobCarryOverTailBytes: Int = record.finalSharedBlobCarryOverTailBytes.toInt(),
  ) = dummyRollupProofResponse(proofIndex).let {
    it.copy(
      publicInputs = it.publicInputs.copy(
        parentDataRollingHash = record.parentDataRollingHash,
        endDataRollingHash = endDataRollingHash,
        parentSharedBlobCarryOverTailBytes = record.parentSharedBlobCarryOverTailBytes.toInt(),
        finalSharedBlobCarryOverTailBytes = finalSharedBlobCarryOverTailBytes,
      ),
    )
  }

  @Test
  fun `a proof matching the request is persisted as the rollup row`() {
    handler.acceptNewRollupProof(proof(), context).get()

    val saved = argumentCaptor<RiscvRollupRecord>()
    verify(rollupsDao).saveRollupWithBlobs(saved.capture())
    assertThat(saved.firstValue).isEqualTo(record)
  }

  @Test
  fun `a proof whose DA outputs differ from the request is not persisted`() {
    assertThatThrownBy { handler.acceptNewRollupProof(proof(endDataRollingHash = Random.nextBytes(32)), context).get() }
      .hasMessageContaining("rollup proof DA outputs differ from the request")
    assertThatThrownBy { handler.acceptNewRollupProof(proof(finalSharedBlobCarryOverTailBytes = 0), context).get() }
      .hasMessageContaining("finalSharedBlobCarryOverTailBytes=0 expected=200")

    verify(rollupsDao, never()).saveRollupWithBlobs(any())
  }
}
