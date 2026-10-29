package lineth.persistence.conflation

import io.vertx.junit5.VertxExtension
import linea.clients.RollupAggregationProofResponseV1
import linea.clients.createRollupAggregationProofResponseV1
import linea.domain.Aggregation
import linea.domain.AggregationV2
import linea.error.DuplicatedRecordException
import linea.persistence.db.DbHelper
import linea.persistence.db.test.CleanDbTestSuiteParallel
import lineth.persistence.test.DbQueries
import net.consensys.FakeFixedClock
import net.consensys.linea.async.get
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.assertThrows
import org.junit.jupiter.api.extension.ExtendWith
import java.util.concurrent.ExecutionException
import kotlin.time.Duration.Companion.seconds
import kotlin.time.Instant

@ExtendWith(VertxExtension::class)
class AggregationsPostgresDaoV2Test : CleanDbTestSuiteParallel() {
  init {
    target = "5"
  }

  override val databaseName = DbHelper.generateUniqueDbName("coordinator-tests-aggregations-dao-v2")

  private val fakeClock = FakeFixedClock(Instant.parse("2026-10-21T00:00:00.000Z"))
  private val blockTime = Instant.parse("2026-10-20T12:00:00Z")
  private lateinit var dao: PostgresAggregationsDaoV2

  @BeforeEach
  fun beforeEach() {
    dao = PostgresAggregationsDaoV2(sqlClient, fakeClock)
  }

  private fun aggregationV2(startBlockNumber: ULong, endBlockNumber: ULong, endBlockTimestamp: Instant = blockTime) =
    AggregationV2(
      startBlockNumber = startBlockNumber,
      endBlockNumber = endBlockNumber,
      batchCount = 3UL,
      aggregationProof = createRollupAggregationProofResponseV1(
        startBlockNumber = startBlockNumber,
        endBlockNumber = endBlockNumber,
        endBlockTimestamp = endBlockTimestamp,
        l2MessagingBlocksOffsets = listOf(1UL, 2UL),
      ),
    )

  private val AggregationV2.proof: RollupAggregationProofResponseV1 get() = aggregationProof!!

  @Test
  fun `saveNewAggregation persists the aggregation row`() {
    val aggregation = aggregationV2(1UL, 10UL)

    dao.saveNewAggregation(aggregation).get()

    val row = DbQueries.getTableContent(sqlClient, DbQueries.aggregationsTable).execute().get().single()
    assertThat(row.getLong("start_block_number")).isEqualTo(1L)
    assertThat(row.getLong("end_block_number")).isEqualTo(10L)
    assertThat(row.getLong("batch_count")).isEqualTo(3L)
    assertThat(row.getInteger("status"))
      .isEqualTo(PostgresAggregationsDaoG.aggregationStatusToDbValue(Aggregation.Status.Proven))
  }

  @Test
  fun `saveNewAggregation fails with DuplicatedRecordException on a duplicate`() {
    val aggregation = aggregationV2(1UL, 10UL)
    dao.saveNewAggregation(aggregation).get()

    val error = assertThrows<ExecutionException> { dao.saveNewAggregation(aggregation).get() }

    assertThat(error.cause).isInstanceOf(DuplicatedRecordException::class.java)
  }

  @Test
  fun `getProofsToFinalize returns consecutive proofs deserialized to the domain`() {
    val aggregation1 = aggregationV2(1UL, 10UL)
    val aggregation2 = aggregationV2(11UL, 20UL)
    val gapped = aggregationV2(22UL, 30UL)
    listOf(aggregation1, aggregation2, gapped).forEach { dao.saveNewAggregation(it).get() }

    assertThat(dao.getProofsToFinalize(1L, blockTime, 10).get())
      .isEqualTo(listOf(aggregation1.proof, aggregation2.proof))
    assertThat(dao.getProofsToFinalize(11L, blockTime, 1).get())
      .isEqualTo(listOf(aggregation2.proof))
  }

  @Test
  fun `getProofsToFinalize filters out proofs whose end block is newer than finalEndBlockCreatedBefore`() {
    val older = aggregationV2(1UL, 10UL, endBlockTimestamp = blockTime)
    val newer = aggregationV2(11UL, 20UL, endBlockTimestamp = blockTime.plus(60.seconds))
    listOf(older, newer).forEach { dao.saveNewAggregation(it).get() }

    assertThat(dao.getProofsToFinalize(1L, blockTime, 10).get()).isEqualTo(listOf(older.proof))
    assertThat(dao.getProofsToFinalize(1L, blockTime.plus(60.seconds), 10).get())
      .isEqualTo(listOf(older.proof, newer.proof))
  }

  @Test
  fun `findAggregationProofByEndBlockNumber returns the proof or null`() {
    val aggregation1 = aggregationV2(1UL, 10UL)
    val aggregation2 = aggregationV2(11UL, 20UL)
    listOf(aggregation1, aggregation2).forEach { dao.saveNewAggregation(it).get() }

    assertThat(dao.findAggregationProofByEndBlockNumber(20L).get()).isEqualTo(aggregation2.proof)
    assertThat(dao.findAggregationProofByEndBlockNumber(15L).get()).isNull()
  }

  @Test
  fun `findHighestConsecutiveEndBlockNumber stops at the first gap`() {
    listOf(aggregationV2(1UL, 10UL), aggregationV2(11UL, 20UL), aggregationV2(22UL, 30UL))
      .forEach { dao.saveNewAggregation(it).get() }

    assertThat(dao.findHighestConsecutiveEndBlockNumber().get()).isEqualTo(20L)
  }
}
