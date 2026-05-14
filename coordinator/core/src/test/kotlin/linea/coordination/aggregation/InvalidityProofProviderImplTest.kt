package linea.coordination.aggregation

import linea.forcedtx.ForcedTransactionInclusionResult
import linea.persistence.ftx.FakeForcedTransactionsDao
import linea.persistence.ftx.ForcedTransactionRecordFactory.createForcedTransactionRecord
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import kotlin.time.Instant

class InvalidityProofProviderImplTest {
  private lateinit var dao: FakeForcedTransactionsDao
  private lateinit var provider: InvalidityProofProviderImpl

  @BeforeEach
  fun setUp() {
    dao = FakeForcedTransactionsDao()
    provider = InvalidityProofProviderImpl(dao)
  }

  private fun saveFtx(
    ftxNumber: ULong,
    simulatedExecutionBlockNumber: ULong,
    inclusionResult: ForcedTransactionInclusionResult = ForcedTransactionInclusionResult.BadNonce,
  ) {
    dao.save(
      createForcedTransactionRecord(
        ftxNumber = ftxNumber,
        simulatedExecutionBlockNumber = simulatedExecutionBlockNumber,
        simulatedExecutionBlockTimestamp = Instant.fromEpochSeconds(simulatedExecutionBlockNumber.toLong()),
        inclusionResult = inclusionResult,
      ),
    ).get()
  }

  @Test
  fun `returns the FTX that opens this aggregation`() {
    // Aggregation starts at block 20, opened by FTX#4.
    saveFtx(ftxNumber = 4UL, simulatedExecutionBlockNumber = 20UL)

    val proofs = provider.getInvalidityProofs(
      ftxStartingNumber = 4UL,
      aggregationStartingBlockNumber = 20UL,
    ).get()

    assertThat(proofs).hasSize(1)
    assertThat(proofs[0].ftxNumber).isEqualTo(4UL)
    assertThat(proofs[0].simulatedExecutionBlockNumber).isEqualTo(20UL)
  }

  @Test
  fun `excludes FTXs whose simulated block is past the aggregation start`() {
    // The current aggregation starts at block 20. FTX#5 at block 23 opens a later aggregation.
    saveFtx(ftxNumber = 4UL, simulatedExecutionBlockNumber = 20UL)
    saveFtx(ftxNumber = 5UL, simulatedExecutionBlockNumber = 23UL)

    val proofs = provider.getInvalidityProofs(
      ftxStartingNumber = 4UL,
      aggregationStartingBlockNumber = 20UL,
    ).get()

    assertThat(proofs.map { it.ftxNumber }).containsExactly(4UL)
  }

  @Test
  fun `excludes FTXs already covered by the parent aggregation`() {
    // Parent aggregation already finalised FTX#3 at block 13. The current aggregation starts
    // at block 20 with FTX#4 — only FTX#4 is in scope.
    saveFtx(ftxNumber = 3UL, simulatedExecutionBlockNumber = 13UL)
    saveFtx(ftxNumber = 4UL, simulatedExecutionBlockNumber = 20UL)

    val proofs = provider.getInvalidityProofs(
      ftxStartingNumber = 4UL,
      aggregationStartingBlockNumber = 20UL,
    ).get()

    assertThat(proofs.map { it.ftxNumber }).containsExactly(4UL)
  }

  @Test
  fun `returns empty when no FTX opens this aggregation`() {
    // FTX#5 belongs to a later aggregation; nothing opens the one starting at block 20.
    saveFtx(ftxNumber = 5UL, simulatedExecutionBlockNumber = 23UL)

    val proofs = provider.getInvalidityProofs(
      ftxStartingNumber = 4UL,
      aggregationStartingBlockNumber = 20UL,
    ).get()

    assertThat(proofs).isEmpty()
  }
}
