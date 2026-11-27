package lineth.coordination.riscv.rollup

import linea.clients.dummyRollupProofResponse
import linea.domain.BlockIntervalProofIndex
import linea.domain.createRiscvRollupRecord
import lineth.persistence.FakeRiscvRollupsDao
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Test
import kotlin.random.Random

class RollupProofPersistenceHandlerTest {
  private val rollupsDao = FakeRiscvRollupsDao()
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

    assertThat(rollupsDao.rollups.values).containsExactly(record)
  }

  @Test
  fun `a proof whose DA outputs differ from the request is not persisted`() {
    assertThatThrownBy { handler.acceptNewRollupProof(proof(endDataRollingHash = Random.nextBytes(32)), context).get() }
      .hasMessageContaining("rollup proof DA outputs differ from the request")
    assertThatThrownBy { handler.acceptNewRollupProof(proof(finalSharedBlobCarryOverTailBytes = 0), context).get() }
      .hasMessageContaining("finalSharedBlobCarryOverTailBytes=0 expected=200")

    assertThat(rollupsDao.rollups).isEmpty()
  }
}
