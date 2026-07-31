<<<<<<< HEAD:coordinator/persistence/src/main/kotlin/lineth/persistence/conflation/BlobsPostgresDaoG.kt
package lineth.persistence.conflation

import io.vertx.sqlclient.Row
import io.vertx.sqlclient.SqlClient
<<<<<<<< HEAD:coordinator/persistence/src/main/kotlin/lineth/persistence/conflation/BlobsPostgresDaoG.kt
=======
package linea.persistence.conflation

import io.vertx.core.Future
import io.vertx.sqlclient.Row
import io.vertx.sqlclient.SqlClient
>>>>>>> 83adcc937 (chore(coordinator): riscv blob data skeleton (#3609)):coordinator/persistence/src/main/kotlin/linea/persistence/conflation/BlobsPostgresDaoG.kt
import io.vertx.sqlclient.Tuple
import linea.domain.BlobStatus
import linea.domain.BlockInterval
import linea.error.DuplicatedRecordException
import linea.persistence.db.SQLQueryLogger
import linea.persistence.db.isDuplicateKeyException
import net.consensys.linea.async.toSafeFuture
import org.apache.logging.log4j.Level
<<<<<<< HEAD:coordinator/persistence/src/main/kotlin/lineth/persistence/conflation/BlobsPostgresDaoG.kt
========
import linea.coordinator.clients.prover.serialization.BlobCompressionProofJsonResponse
import linea.domain.BlobCompressionProof
import linea.domain.BlobRecord
import linea.domain.BlobStatus
import linea.kotlin.decodeHex
import linea.kotlin.encodeHex
import org.apache.logging.log4j.LogManager
>>>>>>>> 83adcc937 (chore(coordinator): riscv blob data skeleton (#3609)):coordinator/persistence/src/main/kotlin/linea/persistence/conflation/BlobsPostgresDao.kt
import org.apache.logging.log4j.Logger
=======
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
>>>>>>> 83adcc937 (chore(coordinator): riscv blob data skeleton (#3609)):coordinator/persistence/src/main/kotlin/linea/persistence/conflation/BlobsPostgresDaoG.kt
import kotlin.time.Clock
import kotlin.time.Instant

abstract class BlobsPostgresDaoG<T : BlockInterval>(
  private val maxBlobsToReturn: UInt,
  connection: SqlClient,
<<<<<<< HEAD:coordinator/persistence/src/main/kotlin/lineth/persistence/conflation/BlobsPostgresDaoG.kt
<<<<<<<< HEAD:coordinator/persistence/src/main/kotlin/lineth/persistence/conflation/BlobsPostgresDaoG.kt
=======
>>>>>>> 83adcc937 (chore(coordinator): riscv blob data skeleton (#3609)):coordinator/persistence/src/main/kotlin/linea/persistence/conflation/BlobsPostgresDaoG.kt
  protected val log: Logger,
  protected val clock: Clock,
) : BlobsDaoG<T> {

  protected val queryLog = SQLQueryLogger(log)

  companion object {
    @JvmStatic
    val TableName = "blobs"

    fun blobStatusToDbValue(status: BlobStatus): Int = when (status) {
      BlobStatus.COMPRESSION_PROVEN -> 1
      BlobStatus.COMPRESSION_PROVING -> 2
    }
  }

  private val selectSql =
    """
      with previous_ends as (select *,
        coalesce(max(end_block_number) over (order by start_block_number asc, end_block_number asc ROWS BETWEEN UNBOUNDED PRECEDING AND 1 PRECEDING), 0) as max_blob_end
        from $TableName
        where start_block_number >= $1 and status = $2
        order by start_block_number asc),
      removed_old_blobs as (select *
        from previous_ends
        where end_block_number > max_blob_end),
      first_gapped_blob as (select start_block_number
        from removed_old_blobs
        where start_block_number > $1 and start_block_number - 1 != max_blob_end
        limit 1)
      select *
      from previous_ends
      where EXISTS (select 1 from $TableName where start_block_number = $1 and status = $2)
        and (previous_ends.max_blob_end = previous_ends.start_block_number - 1 or previous_ends.start_block_number = $1)
        and ((select count(1) from first_gapped_blob) = 0 or previous_ends.start_block_number < (select * from first_gapped_blob))
      limit $maxBlobsToReturn
    """
      .trimIndent()
<<<<<<< HEAD:coordinator/persistence/src/main/kotlin/lineth/persistence/conflation/BlobsPostgresDaoG.kt
========
  log: Logger = LogManager.getLogger(BlobsPostgresDao::class.java),
  clock: Clock = Clock.System,
) : BlobsPostgresDaoG<BlobRecord>(config.maxBlobsToReturn, connection, log, clock), BlobsDao {

  data class Config(val maxBlobsToReturn: UInt)

  companion object {
    fun parseRecord(record: Row): BlobRecord {
      val blobCompressionProof = record.getJsonObject("blob_compression_proof")?.let { jsonObject ->
        BlobCompressionProofJsonResponse.fromJsonString(jsonObject.encode()).toDomainObject()
      }
      return BlobRecord(
        startBlockNumber = record.getLong("start_block_number").toULong(),
        endBlockNumber = record.getLong("end_block_number").toULong(),
        blobHash = record.getString("blob_hash").decodeHex(),
        startBlockTime = Instant.fromEpochMilliseconds(record.getLong("start_block_timestamp")),
        endBlockTime = Instant.fromEpochMilliseconds(record.getLong("end_block_timestamp")),
        batchesCount = record.getInteger("batches_count").toUInt(),
        expectedShnarf = record.getString("expected_shnarf").decodeHex(),
        blobCompressionProof = blobCompressionProof,
      )
    }

    private fun BlobCompressionProof?.toJsonString(): String? =
      this?.let { BlobCompressionProofJsonResponse.fromDomainObject(it).toJsonString() }
  }

  override fun parseRecord(row: Row): BlobRecord = Companion.parseRecord(row)
>>>>>>>> 83adcc937 (chore(coordinator): riscv blob data skeleton (#3609)):coordinator/persistence/src/main/kotlin/linea/persistence/conflation/BlobsPostgresDao.kt

  override fun endBlockTime(record: BlobRecord): Instant = record.endBlockTime

<<<<<<<< HEAD:coordinator/persistence/src/main/kotlin/lineth/persistence/conflation/BlobsPostgresDaoG.kt
=======

  private val selectBlobByEndBlockNumberSql =
    """
      select *
      from $TableName
      where end_block_number = $1
      limit 1
    """
      .trimIndent()

>>>>>>> 83adcc937 (chore(coordinator): riscv blob data skeleton (#3609)):coordinator/persistence/src/main/kotlin/linea/persistence/conflation/BlobsPostgresDaoG.kt
  private val selectBlobByStartBlockNumberSql =
    """
      select *
      from $TableName
      where start_block_number = $1
      limit 1
    """
      .trimIndent()

  // TODO: after riscv migration, drop blob_hash and expected_shnarf columns and rename blob_compression_proof to blobs_info
  // https://github.com/LFDT-Lineth/lineth-monorepo/issues/3658
  protected val insertSql =
    """
     insert into $TableName
     (created_epoch_milli, start_block_number, end_block_number,
     blob_hash, status, start_block_timestamp, end_block_timestamp,
     batches_count, expected_shnarf, blob_compression_proof)
     VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, CAST($10::text as jsonb))
   """
      .trimIndent()

  private val deleteUptoSql =
    """
      delete from $TableName
      where end_block_number <= $1
    """
      .trimIndent()

  private val deleteAfterSql =
    """
      delete from $TableName
      where start_block_number >= $1
    """
      .trimIndent()

  private val selectQuery = connection.preparedQuery(selectSql)
  private val selectBlobByStartBlockNumberQuery = connection.preparedQuery(selectBlobByStartBlockNumberSql)
  private val selectBlobByEndBlockNumberQuery = connection.preparedQuery(selectBlobByEndBlockNumberSql)
  protected val insertQuery = connection.preparedQuery(insertSql)
  private val deleteUptoQuery = connection.preparedQuery(deleteUptoSql)
  private val deleteAfterQuery = connection.preparedQuery(deleteAfterSql)

  protected abstract fun parseRecord(row: Row): T
  protected abstract fun buildInsertParams(blobRecord: T): List<Any?>
  protected abstract fun endBlockTime(record: T): Instant

  override fun saveNewBlob(blobRecord: T): SafeFuture<Unit> {
    val params = buildInsertParams(blobRecord)
    queryLog.log(Level.TRACE, insertSql, params)
    return insertQuery.execute(Tuple.tuple(params))
      .map { }
      .recover { th ->
        if (isDuplicateKeyException(th)) {
          Future.failedFuture(
            DuplicatedRecordException("Blob ${blobRecord.intervalString()} is already persisted!", th),
          )
        } else {
          Future.failedFuture(th)
        }
      }
      .toSafeFuture()
  }

  private fun getConsecutiveBlobsFromBlockNumber(startingBlockNumberInclusive: ULong): SafeFuture<List<T>> {
    return selectQuery
      .execute(
        Tuple.of(
          startingBlockNumberInclusive.toLong(),
          blobStatusToDbValue(BlobStatus.COMPRESSION_PROVEN),
        ),
      )
      .toSafeFuture()
      .thenApply { rowSet -> rowSet.map(::parseRecord) }
  }

  override fun getConsecutiveBlobsFromBlockNumber(
    startingBlockNumberInclusive: ULong,
    endBlockCreatedBefore: Instant,
  ): SafeFuture<List<T>> {
    return getConsecutiveBlobsFromBlockNumber(startingBlockNumberInclusive)
      .thenApply { blobs ->
        blobs.filter { endBlockTime(it) < endBlockCreatedBefore }
      }
  }

  override fun findBlobByStartBlockNumber(startBlockNumber: ULong): SafeFuture<T?> {
    return selectBlobByStartBlockNumberQuery
      .execute(Tuple.of(startBlockNumber.toLong()))
      .toSafeFuture()
      .thenApply { rowSet -> rowSet.map(::parseRecord) }
      .thenApply { it.firstOrNull() }
  }

  override fun findBlobByEndBlockNumber(endBlockNumber: ULong): SafeFuture<T?> {
    return selectBlobByEndBlockNumberQuery
      .execute(Tuple.of(endBlockNumber.toLong()))
      .toSafeFuture()
      .thenApply { rowSet -> rowSet.map(::parseRecord) }
      .thenApply { it.firstOrNull() }
  }

  override fun deleteBlobsUpToEndBlockNumber(endBlockNumberInclusive: ULong): SafeFuture<Int> {
    return deleteUptoQuery
      .execute(Tuple.of(endBlockNumberInclusive.toLong()))
      .map { rowSet -> rowSet.rowCount() }
      .toSafeFuture()
  }

  override fun deleteBlobsAfterBlockNumber(startingBlockNumberInclusive: ULong): SafeFuture<Int> {
    return deleteAfterQuery
      .execute(Tuple.of(startingBlockNumberInclusive.toLong()))
      .map { rowSet -> rowSet.rowCount() }
      .toSafeFuture()
  }
<<<<<<< HEAD:coordinator/persistence/src/main/kotlin/lineth/persistence/conflation/BlobsPostgresDaoG.kt
========
  override fun buildInsertParams(blobRecord: BlobRecord): List<Any?> =
    listOf(
      clock.now().toEpochMilliseconds(),
      blobRecord.startBlockNumber.toLong(),
      blobRecord.endBlockNumber.toLong(),
      blobRecord.blobHash.encodeHex(),
      blobStatusToDbValue(BlobStatus.COMPRESSION_PROVEN),
      blobRecord.startBlockTime.toEpochMilliseconds(),
      blobRecord.endBlockTime.toEpochMilliseconds(),
      blobRecord.batchesCount.toInt(),
      blobRecord.expectedShnarf.encodeHex(),
      blobRecord.blobCompressionProof.toJsonString(),
    )
>>>>>>>> 83adcc937 (chore(coordinator): riscv blob data skeleton (#3609)):coordinator/persistence/src/main/kotlin/linea/persistence/conflation/BlobsPostgresDao.kt
=======
>>>>>>> 83adcc937 (chore(coordinator): riscv blob data skeleton (#3609)):coordinator/persistence/src/main/kotlin/linea/persistence/conflation/BlobsPostgresDaoG.kt
}
