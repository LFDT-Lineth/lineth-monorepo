package lineth.persistence.conflation

import io.vertx.core.Future
import io.vertx.sqlclient.SqlClient
import io.vertx.sqlclient.Tuple
import linea.domain.Aggregation
import linea.domain.AggregationG
import linea.error.DuplicatedRecordException
import linea.persistence.db.SQLQueryLogger
import linea.persistence.db.isDuplicateKeyException
import lineth.persistence.AggregationsDaoG
import net.consensys.linea.async.toSafeFuture
import org.apache.logging.log4j.Level
import org.apache.logging.log4j.LogManager
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Clock
import kotlin.time.Instant

abstract class PostgresAggregationsDaoG<T>(
  connection: SqlClient,
  private val clock: Clock,
) : AggregationsDaoG<T> {

  protected val log = LogManager.getLogger(this.javaClass.name)
  protected val queryLog = SQLQueryLogger(log)

  companion object {
    fun aggregationStatusToDbValue(status: Aggregation.Status): Int {
      return when (status) {
        Aggregation.Status.Proven -> 1
        Aggregation.Status.Proving -> 2
      }
    }

    @JvmStatic
    val aggregationsTable = "aggregations"

    @JvmStatic
    val batchesTable = "batches"

    @JvmStatic
    val blobsTable = "blobs"
  }

  protected abstract fun serializeProof(proof: T?): String?
  protected abstract fun deserializeProof(json: String): T
  protected abstract fun proofFinalTimestamp(proof: T): Instant

  private val insertQuery =
    connection.preparedQuery(
      """
        insert into $aggregationsTable
        (start_block_number, end_block_number, status, start_block_timestamp, batch_count, aggregation_proof)
        VALUES ($1, $2, $3, $4, $5, CAST($6::text as jsonb))
      """.trimIndent(),
    )

  private val selectAggregations =
    connection.preparedQuery(
      """
        with previous_ends as (select *,
          lag(end_block_number, 1) over (order by end_block_number asc) as previous_end_block_number
          from $aggregationsTable
          where start_block_number >= $1 and status = $2
          order by start_block_number asc),
        first_gapped_aggregation as (select start_block_number
          from previous_ends
          where start_block_number > $1 and start_block_number - 1 != previous_end_block_number
          limit 1)

        select * from previous_ends
        where EXISTS (select 1 from previous_ends where start_block_number = $1)
          and (previous_ends.previous_end_block_number = previous_ends.start_block_number - 1 or previous_ends.start_block_number = $1)
          and ((select count(1) from first_gapped_aggregation) = 0 or previous_ends.start_block_number < (select * from first_gapped_aggregation))
        limit $3
      """.trimIndent(),
    )

  private val findAggregationByEndBlockNumber =
    connection.preparedQuery(
      """
        select * from $aggregationsTable
        where end_block_number = $1
        LIMIT 2
      """.trimIndent(),
    )

  private val findFirstAggregation =
    connection.preparedQuery(
      """
        select * from $aggregationsTable
        order by start_block_number asc
        LIMIT 1
      """.trimIndent(),
    )

  private val deleteUptoQuery =
    connection.preparedQuery(
      """
        delete from $aggregationsTable
        where end_block_number <= $1
      """.trimIndent(),
    )

  private val deleteAfterQuery =
    connection.preparedQuery(
      """
        delete from $aggregationsTable
        where start_block_number >= $1
      """.trimIndent(),
    )

  protected fun saveAggregationRecord(aggregation: AggregationG<T>): SafeFuture<Unit> {
    val startBlockNumber = aggregation.startBlockNumber.toLong()
    val endBlockNumber = aggregation.endBlockNumber.toLong()
    val status = aggregationStatusToDbValue(Aggregation.Status.Proven)
    val batchCount = aggregation.batchCount.toLong()
    val aggregationProof = serializeProof(aggregation.aggregationProof)

    val params =
      listOf(
        startBlockNumber,
        endBlockNumber,
        status,
        clock.now().toEpochMilliseconds(),
        batchCount,
        aggregationProof,
      )
    queryLog.log(Level.TRACE, insertQuery.toString(), params)
    return insertQuery.execute(Tuple.tuple(params))
      .map { }
      .recover { th ->
        if (isDuplicateKeyException(th)) {
          Future.failedFuture(
            DuplicatedRecordException(
              "Aggregation startBlockNumber=$startBlockNumber, endBlockNumber=$endBlockNumber " +
                "batchCount=$batchCount is already persisted!",
              th,
            ),
          )
        } else {
          Future.failedFuture(th)
        }
      }
      .toSafeFuture()
  }

  override fun getProofsToFinalize(
    fromBlockNumber: Long,
    finalEndBlockCreatedBefore: Instant,
    maximumNumberOfProofs: Int,
  ): SafeFuture<List<T>> {
    return selectAggregations
      .execute(
        Tuple.of(
          fromBlockNumber,
          aggregationStatusToDbValue(Aggregation.Status.Proven),
          maximumNumberOfProofs,
        ),
      )
      .toSafeFuture()
      .thenApply { rowSet ->
        rowSet
          .map { row -> deserializeProof(row.getJsonObject("aggregation_proof").encode()) }
          .filter { proof -> proofFinalTimestamp(proof) <= finalEndBlockCreatedBefore }
      }
  }

  override fun findHighestConsecutiveEndBlockNumber(fromBlockNumber: Long?): SafeFuture<Long?> {
    return if (fromBlockNumber != null) {
      SafeFuture.completedFuture(fromBlockNumber)
    } else {
      findFirstAggregationStartBlockNumber()
    }.thenCompose { from ->
      if (from != null) {
        selectAggregations
          .execute(
            Tuple.of(
              from,
              aggregationStatusToDbValue(Aggregation.Status.Proven),
              Int.MAX_VALUE,
            ),
          )
          .toSafeFuture()
          .thenApply { rowSet ->
            rowSet.lastOrNull()?.getLong("end_block_number")
          }
      } else {
        SafeFuture.completedFuture(null)
      }
    }
  }

  override fun findAggregationProofByEndBlockNumber(endBlockNumber: Long): SafeFuture<T?> {
    return findAggregationByEndBlockNumber
      .execute(Tuple.of(endBlockNumber))
      .toSafeFuture()
      .thenApply { rowSet ->
        val proofs = rowSet.map { row -> deserializeProof(row.getJsonObject("aggregation_proof").encode()) }
        if (proofs.size > 1) {
          throw IllegalStateException(
            "Multiple aggregations found for endBlockNumber=$endBlockNumber",
          )
        } else {
          proofs.firstOrNull()
        }
      }
  }

  private fun findFirstAggregationStartBlockNumber(): SafeFuture<Long?> {
    return findFirstAggregation
      .execute()
      .toSafeFuture()
      .thenApply { rowSet ->
        rowSet.firstOrNull()?.getLong("start_block_number")
      }
  }

  override fun deleteAggregationsUpToEndBlockNumber(endBlockNumberInclusive: Long): SafeFuture<Int> {
    return deleteUptoQuery
      .execute(Tuple.of(endBlockNumberInclusive))
      .map { rowSet -> rowSet.rowCount() }
      .toSafeFuture()
  }

  override fun deleteAggregationsAfterBlockNumber(startingBlockNumberInclusive: Long): SafeFuture<Int> {
    return deleteAfterQuery
      .execute(Tuple.of(startingBlockNumberInclusive))
      .map { rowSet -> rowSet.rowCount() }
      .toSafeFuture()
  }
}
