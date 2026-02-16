package linea.persistence.ftx

import io.vertx.junit5.VertxExtension
import io.vertx.sqlclient.PreparedQuery
import io.vertx.sqlclient.Row
import io.vertx.sqlclient.RowSet
import linea.forcedtx.ForcedTransactionInclusionResult
import linea.persistence.ftx.PostgresForcedTransactionsDao.Companion.dbValueToInclusionResult
import linea.persistence.ftx.PostgresForcedTransactionsDao.Companion.dbValueToProofStatus
import linea.persistence.ftx.PostgresForcedTransactionsDao.Companion.inclusionResultToDbValue
import linea.persistence.ftx.PostgresForcedTransactionsDao.Companion.proofStatusToDbValue
import net.consensys.FakeFixedClock
import net.consensys.linea.async.get
import net.consensys.zkevm.domain.ForcedTransactionRecord
import net.consensys.zkevm.persistence.db.DbHelper
import net.consensys.zkevm.persistence.db.test.CleanDbTestSuiteParallel
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Duration.Companion.seconds
import kotlin.time.Instant

@ExtendWith(VertxExtension::class)
class PostgresForcedTransactionsDaoTest : CleanDbTestSuiteParallel() {
  init {
    target = "5"
  }

  override val databaseName = DbHelper.generateUniqueDbName(prefix = "coordinator-tests-forced-transactions")
  private var fakeClock = FakeFixedClock(Instant.parse("2026-02-03T00:00:00.000Z"))

  private fun forcedTransactionsContentQuery(): PreparedQuery<RowSet<Row>> =
    sqlClient.preparedQuery("select * from forced_transactions")

  private lateinit var forcedTransactionsDao: ForcedTransactionsDao

  @BeforeEach
  fun beforeEach() {
    forcedTransactionsDao =
      PostgresForcedTransactionsDao(
        sqlClient,
        fakeClock,
      )
  }

  @AfterEach
  override fun tearDown() {
    super.tearDown()
  }

  private fun createForcedTransactionRecord(
    ftxNumber: ULong,
    inclusionResult: ForcedTransactionInclusionResult = ForcedTransactionInclusionResult.BadNonce,
    simulatedExecutionBlockNumber: ULong = 1000UL,
    simulatedExecutionBlockTimestamp: Instant = Instant.fromEpochMilliseconds(1705276800000),
    proofStatus: ForcedTransactionRecord.ProofStatus = ForcedTransactionRecord.ProofStatus.UNREQUESTED,
  ): ForcedTransactionRecord {
    return ForcedTransactionRecord(
      ftxNumber = ftxNumber,
      inclusionResult = inclusionResult,
      simulatedExecutionBlockNumber = simulatedExecutionBlockNumber,
      simulatedExecutionBlockTimestamp = simulatedExecutionBlockTimestamp,
      proofStatus = proofStatus,
      proofIndex = null,
    )
  }

  @Test
  fun `save inserts a new forced transaction record to the database`() {
    val ftx1 = createForcedTransactionRecord(
      ftxNumber = 123UL,
      inclusionResult = ForcedTransactionInclusionResult.BadNonce,
      proofStatus = ForcedTransactionRecord.ProofStatus.UNREQUESTED,
    )

    forcedTransactionsDao.save(ftx1).get()

    val dbContent = forcedTransactionsContentQuery().execute().get()
    assertThat(dbContent).hasSize(1)

    val row = dbContent.first()
    assertThat(row.getLong("created_epoch_milli")).isEqualTo(fakeClock.now().toEpochMilliseconds())
    assertThat(row.getLong("updated_epoch_milli")).isEqualTo(fakeClock.now().toEpochMilliseconds())
    assertThat(row.getLong("ftx_number")).isEqualTo(ftx1.ftxNumber.toLong())
    assertThat(row.getShort("inclusion_result"))
      .isEqualTo(inclusionResultToDbValue(ForcedTransactionInclusionResult.BadNonce).toShort())
    assertThat(row.getLong("simulated_execution_block_number")).isEqualTo(ftx1.simulatedExecutionBlockNumber.toLong())
    assertThat(row.getShort("proof_status"))
      .isEqualTo(proofStatusToDbValue(ForcedTransactionRecord.ProofStatus.UNREQUESTED).toShort())
  }

  @Test
  fun `save updates an existing forced transaction record on conflict`() {
    val ftx1 = createForcedTransactionRecord(
      ftxNumber = 456UL,
      inclusionResult = ForcedTransactionInclusionResult.BadNonce,
      proofStatus = ForcedTransactionRecord.ProofStatus.UNREQUESTED,
    )

    forcedTransactionsDao.save(ftx1).get()
    fakeClock.advanceBy(10.seconds)

    // Update with different values
    val ftx1Updated = createForcedTransactionRecord(
      ftxNumber = 456UL,
      inclusionResult = ForcedTransactionInclusionResult.BadBalance,
      proofStatus = ForcedTransactionRecord.ProofStatus.REQUESTED,
      simulatedExecutionBlockNumber = 2000UL,
    )

    forcedTransactionsDao.save(ftx1Updated).get()

    val dbContent = forcedTransactionsContentQuery().execute().get()
    assertThat(dbContent).hasSize(1)

    val row = dbContent.first()
    assertThat(row.getLong("ftx_number")).isEqualTo(ftx1Updated.ftxNumber.toLong())
    assertThat(row.getShort("inclusion_result"))
      .isEqualTo(inclusionResultToDbValue(ForcedTransactionInclusionResult.BadBalance).toShort())
    assertThat(row.getLong("simulated_execution_block_number"))
      .isEqualTo(ftx1Updated.simulatedExecutionBlockNumber.toLong())
    assertThat(row.getShort("proof_status"))
      .isEqualTo(
        proofStatusToDbValue(ForcedTransactionRecord.ProofStatus.REQUESTED).toShort(),
      )
    assertThat(row.getLong("updated_epoch_milli")).isEqualTo(fakeClock.now().toEpochMilliseconds())
  }

  @Test
  fun `save correctly stores all inclusion result enum values`() {
    val inclusionResults = listOf(
      ForcedTransactionInclusionResult.Included,
      ForcedTransactionInclusionResult.BadNonce,
      ForcedTransactionInclusionResult.BadBalance,
      ForcedTransactionInclusionResult.BadPrecompile,
      ForcedTransactionInclusionResult.TooManyLogs,
      ForcedTransactionInclusionResult.FilteredAddressFrom,
      ForcedTransactionInclusionResult.FilteredAddressTo,
      ForcedTransactionInclusionResult.Phylax,
    )

    inclusionResults.forEachIndexed { index, inclusionResult ->
      val ftx = createForcedTransactionRecord(
        ftxNumber = index.toULong(),
        inclusionResult = inclusionResult,
      )
      forcedTransactionsDao.save(ftx).get()
    }

    val dbContent = forcedTransactionsContentQuery().execute().get()
    assertThat(dbContent).hasSize(inclusionResults.size)

    dbContent.sortedBy { it.getLong("ftx_number") }.forEachIndexed { index, row ->
      assertThat(row.getShort("inclusion_result"))
        .isEqualTo(inclusionResultToDbValue(inclusionResults[index]).toShort())
    }
  }

  @Test
  fun `save correctly stores all proof status enum values`() {
    val proofStatuses = listOf(
      ForcedTransactionRecord.ProofStatus.UNREQUESTED,
      ForcedTransactionRecord.ProofStatus.REQUESTED,
      ForcedTransactionRecord.ProofStatus.PROVEN,
    )

    proofStatuses.forEachIndexed { index, proofStatus ->
      val ftx = createForcedTransactionRecord(
        ftxNumber = index.toULong(),
        proofStatus = proofStatus,
      )
      forcedTransactionsDao.save(ftx).get()
    }

    val dbContent = forcedTransactionsContentQuery().execute().get()
    assertThat(dbContent).hasSize(proofStatuses.size)

    dbContent.sortedBy { it.getLong("ftx_number") }.forEachIndexed { index, row ->
      assertThat(row.getShort("proof_status"))
        .isEqualTo(PostgresForcedTransactionsDao.proofStatusToDbValue(proofStatuses[index]).toShort())
    }
  }

  @Test
  fun `findByNumber returns null when record does not exist`() {
    val result = forcedTransactionsDao.findByNumber(999UL).get()
    assertThat(result).isNull()
  }

  @Test
  fun `findByNumber returns the correct record when it exists`() {
    val ftx = createForcedTransactionRecord(
      ftxNumber = 789UL,
      inclusionResult = ForcedTransactionInclusionResult.TooManyLogs,
      proofStatus = ForcedTransactionRecord.ProofStatus.REQUESTED,
      simulatedExecutionBlockNumber = 5000UL,
    )

    forcedTransactionsDao.save(ftx).get()

    val result = forcedTransactionsDao.findByNumber(789UL).get()
    assertThat(result).isNotNull
    assertThat(result!!.ftxNumber).isEqualTo(789UL)
    assertThat(result.inclusionResult).isEqualTo(ForcedTransactionInclusionResult.TooManyLogs)
    assertThat(result.proofStatus).isEqualTo(ForcedTransactionRecord.ProofStatus.REQUESTED)
    assertThat(result.simulatedExecutionBlockNumber).isEqualTo(5000UL)
    assertThat(result.proofIndex).isNull()
  }

  @Test
  fun `list returns empty list when no records exist`() {
    val result = forcedTransactionsDao.list().get()
    assertThat(result).isEmpty()
  }

  @Test
  fun `list returns all records sorted by ftx_number`() {
    val ftxRecords = listOf(
      createForcedTransactionRecord(ftxNumber = 300UL, inclusionResult = ForcedTransactionInclusionResult.BadNonce),
      createForcedTransactionRecord(ftxNumber = 100UL, inclusionResult = ForcedTransactionInclusionResult.Included),
      createForcedTransactionRecord(ftxNumber = 200UL, inclusionResult = ForcedTransactionInclusionResult.BadBalance),
    )

    SafeFuture.collectAll(ftxRecords.map { forcedTransactionsDao.save(it) }.stream()).get()

    val result = forcedTransactionsDao.list().get()
    assertThat(result).hasSize(3)
    assertThat(result.map { it.ftxNumber }).containsExactly(100UL, 200UL, 300UL)
    assertThat(result).isEqualTo(ftxRecords.sortedBy { it.ftxNumber })
  }

  @Test
  fun `deleteFtxUpToInclusive deletes no records when none match`() {
    val ftxRecords = listOf(
      createForcedTransactionRecord(ftxNumber = 100UL),
      createForcedTransactionRecord(ftxNumber = 200UL),
      createForcedTransactionRecord(ftxNumber = 300UL),
    )

    SafeFuture.collectAll(ftxRecords.map { forcedTransactionsDao.save(it) }.stream()).get()

    val deletedCount = forcedTransactionsDao.deleteFtxUpToInclusive(50UL).get()
    assertThat(deletedCount).isEqualTo(0)

    val remaining = forcedTransactionsDao.list().get()
    assertThat(remaining).hasSize(3)
  }

  @Test
  fun `deleteFtxUpToInclusive deletes all records up to and including the specified ftx number`() {
    val ftxRecords = listOf(
      createForcedTransactionRecord(ftxNumber = 100UL),
      createForcedTransactionRecord(ftxNumber = 200UL),
      createForcedTransactionRecord(ftxNumber = 300UL),
      createForcedTransactionRecord(ftxNumber = 400UL),
    )

    SafeFuture.collectAll(ftxRecords.map { forcedTransactionsDao.save(it) }.stream()).get()

    val deletedCount = forcedTransactionsDao.deleteFtxUpToInclusive(250UL).get()
    assertThat(deletedCount).isEqualTo(2)

    val remaining = forcedTransactionsDao.list().get()
    assertThat(remaining).hasSize(2)
    assertThat(remaining.map { it.ftxNumber }).containsExactly(300UL, 400UL)
  }

  @Test
  fun `deleteFtxUpToInclusive deletes all records when ftx number is greater than all`() {
    val ftxRecords = listOf(
      createForcedTransactionRecord(ftxNumber = 100UL),
      createForcedTransactionRecord(ftxNumber = 200UL),
      createForcedTransactionRecord(ftxNumber = 300UL),
    )

    SafeFuture.collectAll(ftxRecords.map { forcedTransactionsDao.save(it) }.stream()).get()

    val deletedCount = forcedTransactionsDao.deleteFtxUpToInclusive(500UL).get()
    assertThat(deletedCount).isEqualTo(3)

    val remaining = forcedTransactionsDao.list().get()
    assertThat(remaining).isEmpty()
  }

  @Test
  fun `enum mapping functions correctly convert between enum and db values`() {
    // Test inclusion result mappings
    assertThat(inclusionResultToDbValue(ForcedTransactionInclusionResult.Included)).isEqualTo(1)
    assertThat(inclusionResultToDbValue(ForcedTransactionInclusionResult.BadNonce)).isEqualTo(2)
    assertThat(inclusionResultToDbValue(ForcedTransactionInclusionResult.BadBalance)).isEqualTo(3)
    assertThat(inclusionResultToDbValue(ForcedTransactionInclusionResult.BadPrecompile)).isEqualTo(4)
    assertThat(inclusionResultToDbValue(ForcedTransactionInclusionResult.TooManyLogs)).isEqualTo(5)
    assertThat(inclusionResultToDbValue(ForcedTransactionInclusionResult.FilteredAddressFrom)).isEqualTo(6)
    assertThat(inclusionResultToDbValue(ForcedTransactionInclusionResult.FilteredAddressTo)).isEqualTo(7)
    assertThat(inclusionResultToDbValue(ForcedTransactionInclusionResult.Phylax)).isEqualTo(8)

    assertThat(dbValueToInclusionResult(1)).isEqualTo(ForcedTransactionInclusionResult.Included)
    assertThat(dbValueToInclusionResult(2)).isEqualTo(ForcedTransactionInclusionResult.BadNonce)
    assertThat(dbValueToInclusionResult(3)).isEqualTo(ForcedTransactionInclusionResult.BadBalance)
    assertThat(dbValueToInclusionResult(4)).isEqualTo(ForcedTransactionInclusionResult.BadPrecompile)
    assertThat(dbValueToInclusionResult(5)).isEqualTo(ForcedTransactionInclusionResult.TooManyLogs)
    assertThat(dbValueToInclusionResult(6)).isEqualTo(ForcedTransactionInclusionResult.FilteredAddressFrom)
    assertThat(dbValueToInclusionResult(7)).isEqualTo(ForcedTransactionInclusionResult.FilteredAddressTo)
    assertThat(dbValueToInclusionResult(8)).isEqualTo(ForcedTransactionInclusionResult.Phylax)

    // Test proof status mappings
    assertThat(proofStatusToDbValue(ForcedTransactionRecord.ProofStatus.UNREQUESTED)).isEqualTo(1)
    assertThat(proofStatusToDbValue(ForcedTransactionRecord.ProofStatus.REQUESTED)).isEqualTo(2)
    assertThat(proofStatusToDbValue(ForcedTransactionRecord.ProofStatus.PROVEN)).isEqualTo(3)

    assertThat(dbValueToProofStatus(1)).isEqualTo(ForcedTransactionRecord.ProofStatus.UNREQUESTED)
    assertThat(dbValueToProofStatus(2)).isEqualTo(ForcedTransactionRecord.ProofStatus.REQUESTED)
    assertThat(dbValueToProofStatus(3)).isEqualTo(ForcedTransactionRecord.ProofStatus.PROVEN)
  }
}
