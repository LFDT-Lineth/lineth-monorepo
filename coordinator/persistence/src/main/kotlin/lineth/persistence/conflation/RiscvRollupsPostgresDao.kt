package lineth.persistence.conflation

import io.vertx.core.Future
import io.vertx.core.buffer.Buffer
import io.vertx.sqlclient.PreparedQuery
import io.vertx.sqlclient.Row
import io.vertx.sqlclient.RowSet
import io.vertx.sqlclient.SqlClient
import io.vertx.sqlclient.Tuple
import linea.domain.BlobData
import linea.domain.RiscvRollupRecord
import linea.error.DuplicatedRecordException
import linea.persistence.db.SQLQueryLogger
import linea.persistence.db.isDuplicateKeyException
import lineth.persistence.RiscvRollupsDao
import net.consensys.linea.async.toSafeFuture
import org.apache.logging.log4j.Level
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Clock
import kotlin.time.Instant

/**
 * RISC-V rollup proofs. Each rollup is a row in [RollupsTableName]; its blobs are stored once each in
 * [BlobsTableName] and listed, in order, in [RollupBlobsTableName]. Adjacent rollups can share a blob, which is then
 * stored once and referenced by both.
 *
 * Deleting rollups cascades to their references; blobs no rollup references any more are then deleted. If that orphan
 * delete races a save that found its shared blob already stored, the save fails its foreign-key check and is retried
 * by the caller, which stores the blob again.
 */
class RiscvRollupsPostgresDao(
  config: Config,
  connection: SqlClient,
  private val log: Logger = LogManager.getLogger(RiscvRollupsPostgresDao::class.java),
  private val clock: Clock = Clock.System,
) : RiscvRollupsDao {

  /**
   * @property ignoreDuplicates when true, saving a rollup that is already stored succeeds without changes; when false
   *   it fails with [DuplicatedRecordException]
   */
  data class Config(
    val maxRollupProofsToReturn: UInt,
    val ignoreDuplicates: Boolean = true,
  )

  companion object {
    const val RollupsTableName = "riscv_rollups"
    const val BlobsTableName = "riscv_blobs"
    const val RollupBlobsTableName = "riscv_rollup_blobs"
  }

  private val queryLog = SQLQueryLogger(log)
  private val ignoreDuplicates = config.ignoreDuplicates

  // One statement, so the rollup, its references and its new blobs are stored atomically. A blob that is already
  // stored (the shared first blob) is kept as is.
  private val insertSql =
    """
      with inserted_blobs as (
        insert into $BlobsTableName
        (data_rolling_hash, parent_data_rolling_hash, chunk_hash, blob_bytes, created_epoch_milli)
        select data_rolling_hash, parent_data_rolling_hash, chunk_hash, blob_bytes, $1
        from unnest($13::bytea[], $14::bytea[], $15::bytea[], $16::bytea[])
          as b(data_rolling_hash, parent_data_rolling_hash, chunk_hash, blob_bytes)
        on conflict (data_rolling_hash) do nothing
      ),
      inserted_references as (
        insert into $RollupBlobsTableName (start_block_number, end_block_number, position_in_rollup, data_rolling_hash)
        select $2, $3, r.ordinality - 1, r.data_rolling_hash
        from unnest($13::bytea[]) with ordinality as r(data_rolling_hash, ordinality)
      )
      insert into $RollupsTableName
      (created_epoch_milli, start_block_number, end_block_number, start_block_timestamp, end_block_timestamp,
      batches_count, proof_hash, parent_data_rolling_hash, data_rolling_hash,
      parent_shared_blob_carry_over_tail_bytes, parent_blob_data_rolling_hash, final_shared_blob_carry_over_tail_bytes)
      values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
    """
      .trimIndent()

  // The gap-free run of rollups starting at $1: rollups never overlap, so each next rollup starts right after the
  // previous one ends, up to the first gap.
  private val selectConsecutiveSql =
    """
      with ordered as (select *,
          lag(end_block_number) over (order by start_block_number) as previous_end_block_number
        from $RollupsTableName
        where start_block_number >= $1),
      first_gap as (select min(start_block_number) as start_block_number
        from ordered
        where start_block_number > $1 and start_block_number != previous_end_block_number + 1)
      select *
      from ordered
      where exists (select 1 from ordered where start_block_number = $1)
        and ((select start_block_number from first_gap) is null
          or start_block_number < (select start_block_number from first_gap))
      order by start_block_number
      limit ${config.maxRollupProofsToReturn}
    """
      .trimIndent()

  private val selectByStartBlockNumberSql = "select * from $RollupsTableName where start_block_number = $1 limit 1"
  private val selectByEndBlockNumberSql = "select * from $RollupsTableName where end_block_number = $1 limit 1"
  private val existsSql =
    "select 1 from $RollupsTableName where start_block_number = $1 and end_block_number = $2"

  private val selectRollupBlobsSql =
    """
      select r.start_block_number, r.end_block_number, b.*
      from $RollupBlobsTableName r
      join $BlobsTableName b on b.data_rolling_hash = r.data_rolling_hash
      where (r.start_block_number, r.end_block_number) in (select * from unnest($1::bigint[], $2::bigint[]))
      order by r.start_block_number, r.end_block_number, r.position_in_rollup
    """
      .trimIndent()

  private val deleteUpToSql = "delete from $RollupsTableName where end_block_number <= $1"
  private val deleteAfterSql = "delete from $RollupsTableName where start_block_number >= $1"
  private val deleteUnreferencedBlobsSql =
    """
      delete from $BlobsTableName b
      where not exists (select 1 from $RollupBlobsTableName r where r.data_rolling_hash = b.data_rolling_hash)
    """
      .trimIndent()

  private val insertQuery = connection.preparedQuery(insertSql)
  private val selectConsecutiveQuery = connection.preparedQuery(selectConsecutiveSql)
  private val selectByStartBlockNumberQuery = connection.preparedQuery(selectByStartBlockNumberSql)
  private val selectByEndBlockNumberQuery = connection.preparedQuery(selectByEndBlockNumberSql)
  private val existsQuery = connection.preparedQuery(existsSql)
  private val selectRollupBlobsQuery = connection.preparedQuery(selectRollupBlobsSql)
  private val deleteUpToQuery = connection.preparedQuery(deleteUpToSql)
  private val deleteAfterQuery = connection.preparedQuery(deleteAfterSql)
  private val deleteUnreferencedBlobsQuery = connection.preparedQuery(deleteUnreferencedBlobsSql)

  override fun saveRollupWithBlobs(rollup: RiscvRollupRecord): SafeFuture<Unit> {
    val blobs = rollup.blobsData
    val params = listOf(
      clock.now().toEpochMilliseconds(),
      rollup.startBlockNumber.toLong(),
      rollup.endBlockNumber.toLong(),
      rollup.startBlockTimestamp.toEpochMilliseconds(),
      rollup.endBlockTimestamp.toEpochMilliseconds(),
      rollup.totalBatchesCount.toInt(),
      Buffer.buffer(rollup.proofHash),
      Buffer.buffer(rollup.parentDataRollingHash),
      Buffer.buffer(rollup.dataRollingHash),
      rollup.parentSharedBlobCarryOverTailBytes.toInt(),
      rollup.parentBlobDataRollingHash?.let(Buffer::buffer),
      rollup.finalSharedBlobCarryOverTailBytes.toInt(),
      blobs.map { Buffer.buffer(it.dataRollingHash) }.toTypedArray(),
      blobs.map { Buffer.buffer(it.prevDataRollingHash) }.toTypedArray(),
      blobs.map { Buffer.buffer(it.chunkHash) }.toTypedArray(),
      blobs.map { Buffer.buffer(it.blobBytes) }.toTypedArray(),
    )
    queryLog.log(Level.TRACE, insertSql, params)
    return insertQuery.execute(Tuple.tuple(params))
      .map { }
      .recover { th -> if (isDuplicateKeyException(th)) duplicateOrConflict(rollup, th) else Future.failedFuture(th) }
      .toSafeFuture()
  }

  /**
   * A unique violation is a duplicate only if this rollup, by block range, is already stored: ignored or failed with
   * [DuplicatedRecordException] per [Config.ignoreDuplicates]. Otherwise the rollup
   * overlaps a stored one (same start or end block), which is a bug: the Postgres error is returned as is, so callers
   * do not ignore it and retries stop.
   */
  private fun duplicateOrConflict(rollup: RiscvRollupRecord, error: Throwable): Future<Unit> =
    existsQuery
      .execute(Tuple.of(rollup.startBlockNumber.toLong(), rollup.endBlockNumber.toLong()))
      .compose { rows ->
        when {
          rows.size() == 0 -> {
            log.error("rollup={} overlaps a stored rollup: {}", rollup.intervalString(), error.message)
            Future.failedFuture(error)
          }
          ignoreDuplicates -> Future.succeededFuture()
          else -> Future.failedFuture(
            DuplicatedRecordException("Rollup ${rollup.intervalString()} is already persisted!", error),
          )
        }
      }

  override fun getConsecutiveRollupsFromBlockNumber(
    startingBlockNumberInclusive: ULong,
    endBlockCreatedBefore: Instant,
  ): SafeFuture<List<RiscvRollupRecord>> =
    selectConsecutiveQuery
      .execute(Tuple.of(startingBlockNumberInclusive.toLong()))
      .toSafeFuture()
      .thenApply { rows -> rows.filter { endBlockTimestamp(it) < endBlockCreatedBefore } }
      .thenCompose(::toRecords)

  override fun findRollupByStartBlockNumber(startBlockNumber: ULong): SafeFuture<RiscvRollupRecord?> =
    selectByStartBlockNumberQuery
      .execute(Tuple.of(startBlockNumber.toLong()))
      .toSafeFuture()
      .thenCompose { rows -> toRecords(rows.toList()) }
      .thenApply { it.firstOrNull() }

  override fun findRollupByEndBlockNumber(endBlockNumber: ULong): SafeFuture<RiscvRollupRecord?> =
    selectByEndBlockNumberQuery
      .execute(Tuple.of(endBlockNumber.toLong()))
      .toSafeFuture()
      .thenCompose { rows -> toRecords(rows.toList()) }
      .thenApply { it.firstOrNull() }

  override fun deleteRollupsUpToEndBlockNumber(endBlockNumberInclusive: ULong): SafeFuture<Int> =
    deleteRollups(deleteUpToQuery, endBlockNumberInclusive)

  override fun deleteRollupsAfterBlockNumber(startingBlockNumberInclusive: ULong): SafeFuture<Int> =
    deleteRollups(deleteAfterQuery, startingBlockNumberInclusive)

  private fun deleteRollups(query: PreparedQuery<RowSet<Row>>, blockNumber: ULong): SafeFuture<Int> =
    query
      .execute(Tuple.of(blockNumber.toLong()))
      .map { rowSet -> rowSet.rowCount() }
      .toSafeFuture()
      .thenCompose { deleted ->
        deleteUnreferencedBlobsQuery.execute().toSafeFuture().thenApply { deleted }
      }

  private fun endBlockTimestamp(row: Row): Instant = Instant.fromEpochMilliseconds(row.getLong("end_block_timestamp"))

  /** Builds the records of [rollupRows], in order, loading their blobs with one query. */
  private fun toRecords(rollupRows: List<Row>): SafeFuture<List<RiscvRollupRecord>> {
    if (rollupRows.isEmpty()) return SafeFuture.completedFuture(emptyList())
    val params = listOf(
      rollupRows.map { it.getLong("start_block_number") }.toTypedArray(),
      rollupRows.map { it.getLong("end_block_number") }.toTypedArray(),
    )
    queryLog.log(Level.TRACE, selectRollupBlobsSql, params)
    return selectRollupBlobsQuery
      .execute(Tuple.tuple(params))
      .toSafeFuture()
      .thenApply { blobRows ->
        val blobsByRollup = blobRows.groupBy(
          keySelector = { it.getLong("start_block_number") to it.getLong("end_block_number") },
          valueTransform = ::parseBlobData,
        )
        rollupRows.map { row ->
          parseRecord(row, blobsByRollup[row.getLong("start_block_number") to row.getLong("end_block_number")])
        }
      }
  }

  private fun parseBlobData(row: Row): BlobData =
    BlobData(
      chunkHash = row.getBuffer("chunk_hash").bytes,
      blobBytes = row.getBuffer("blob_bytes").bytes,
      prevDataRollingHash = row.getBuffer("parent_data_rolling_hash").bytes,
      dataRollingHash = row.getBuffer("data_rolling_hash").bytes,
    )

  private fun parseRecord(row: Row, blobsData: List<BlobData>?): RiscvRollupRecord {
    require(!blobsData.isNullOrEmpty()) {
      "no blobs stored for RISC-V rollup at blocks " +
        "${row.getLong("start_block_number")}..${row.getLong("end_block_number")}"
    }
    return RiscvRollupRecord(
      startBlockNumber = row.getLong("start_block_number").toULong(),
      endBlockNumber = row.getLong("end_block_number").toULong(),
      startBlockTimestamp = Instant.fromEpochMilliseconds(row.getLong("start_block_timestamp")),
      endBlockTimestamp = endBlockTimestamp(row),
      totalBatchesCount = row.getInteger("batches_count").toUInt(),
      parentDataRollingHash = row.getBuffer("parent_data_rolling_hash").bytes,
      dataRollingHash = row.getBuffer("data_rolling_hash").bytes,
      parentSharedBlobCarryOverTailBytes = row.getInteger("parent_shared_blob_carry_over_tail_bytes").toUInt(),
      parentBlobDataRollingHash = row.getBuffer("parent_blob_data_rolling_hash")?.bytes,
      finalSharedBlobCarryOverTailBytes = row.getInteger("final_shared_blob_carry_over_tail_bytes").toUInt(),
      blobsData = blobsData,
      proofHash = row.getBuffer("proof_hash").bytes,
    )
  }
}
