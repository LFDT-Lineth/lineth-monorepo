package net.consensys.zkevm.persistence.dao.batch.persistence

import io.vertx.core.Future
import io.vertx.pgclient.PgException
import io.vertx.sqlclient.SqlClient
import io.vertx.sqlclient.Tuple
import net.consensys.linea.async.toSafeFuture
import net.consensys.zkevm.domain.Batch
import net.consensys.zkevm.persistence.db.DuplicatedRecordException
import net.consensys.zkevm.persistence.db.SQLQueryLogger
import org.apache.logging.log4j.Level
import org.apache.logging.log4j.LogManager
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Clock

class BatchesPostgresDao(
  connection: SqlClient,
  private val clock: Clock = Clock.System,
) : BatchesDao {
  private val log = LogManager.getLogger(this.javaClass.name)
  private val queryLog = SQLQueryLogger(log)

  companion object {
    /**
     * WARNING: Existing mappings should not change. Otherwise, can break production, new one can be added
     * though.
     */
    fun batchStatusToDbValue(status: Batch.Status): Int {
      // using manual mapping to catch errors at compile time instead of runtime
      return when (status) {
        Batch.Status.Finalized -> 1
        Batch.Status.Proven -> 2
      }
    }

    @JvmStatic
    val batchesTableName = "batches"
  }

  private val insertSql =
    """
      insert into $batchesTableName
      (created_epoch_milli, start_block_number, end_block_number, status)
      VALUES ($1, $2, $3, $4)
    """
      .trimIndent()

  private val findHighestConsecutiveEndBlockNumberSql =
    """
      with ranked_versions as (select *,
              dense_rank() over (partition by start_block_number order by end_block_number asc) version_rank
              from $batchesTableName
              order by start_block_number asc),
          previous_ends as (select *,
              lag(end_block_number, 1) over (order by end_block_number asc) as previous_end_block_number
              from ranked_versions
              where version_rank = 1),
           first_gapped_batch as (select start_block_number
              from previous_ends
              where start_block_number > $1 and start_block_number - 1 != previous_end_block_number
              limit 1)
      select end_block_number
      from previous_ends
      where EXISTS (select 1 from $batchesTableName where start_block_number = $1)
        and (previous_ends.previous_end_block_number = previous_ends.start_block_number - 1 or previous_ends.start_block_number = $1)
        and ((select count(1) from first_gapped_batch) = 0 or previous_ends.start_block_number < (select * from first_gapped_batch))
      order by start_block_number desc
      limit 1
    """
      .trimIndent()

  private val deleteUptoSql =
    """
        delete from $batchesTableName
        where end_block_number <= $1
    """
      .trimIndent()

  private val deleteAfterSql =
    """
        delete from $batchesTableName
        where start_block_number >= $1
    """
      .trimIndent()

  private val findHighestConsecutiveEndBlockNumberQuery = connection.preparedQuery(
    findHighestConsecutiveEndBlockNumberSql,
  )
  private val insertQuery = connection.preparedQuery(insertSql)
  private val deleteUptoQuery = connection.preparedQuery(deleteUptoSql)
  private val deleteAfterQuery = connection.preparedQuery(deleteAfterSql)

  override fun saveNewBatch(batch: Batch): SafeFuture<Unit> {
    val startBlockNumber = batch.startBlockNumber.toLong()
    val endBlockNumber = batch.endBlockNumber.toLong()
    val params =
      listOf(
        clock.now().toEpochMilliseconds(),
        startBlockNumber,
        endBlockNumber,
        batchStatusToDbValue(Batch.Status.Proven),
      )
    queryLog.log(Level.TRACE, insertSql, params)
    return insertQuery.execute(Tuple.tuple(params))
      .map { }
      .recover { th ->
        if (th is PgException && th.errorMessage == "duplicate key value violates unique constraint \"batches_pkey\"") {
          Future.failedFuture(
            DuplicatedRecordException(
              "Batch startBlockNumber=$startBlockNumber, endBlockNumber=$endBlockNumber " +
                "is already persisted!",
              th,
            ),
          )
        } else {
          Future.failedFuture(th)
        }
      }
      .toSafeFuture()
  }

  override fun findHighestConsecutiveEndBlockNumberFromBlockNumber(
    startingBlockNumberInclusive: Long,
  ): SafeFuture<Long?> {
    val params = listOf(startingBlockNumberInclusive)
    queryLog.log(Level.TRACE, findHighestConsecutiveEndBlockNumberSql, params)
    return findHighestConsecutiveEndBlockNumberQuery
      .execute(Tuple.tuple(params))
      .toSafeFuture()
      .thenApply { rowSet ->
        rowSet.firstOrNull()?.getLong("end_block_number")
      }
  }

  override fun deleteBatchesUpToEndBlockNumber(endBlockNumberInclusive: Long): SafeFuture<Int> {
    return deleteUptoQuery
      .execute(
        Tuple.of(
          endBlockNumberInclusive,
        ),
      )
      .map { rowSet -> rowSet.rowCount() }
      .toSafeFuture()
  }

  override fun deleteBatchesAfterBlockNumber(startingBlockNumberInclusive: Long): SafeFuture<Int> {
    return deleteAfterQuery
      .execute(
        Tuple.of(
          startingBlockNumberInclusive,
        ),
      )
      .map { rowSet -> rowSet.rowCount() }
      .toSafeFuture()
  }
}
