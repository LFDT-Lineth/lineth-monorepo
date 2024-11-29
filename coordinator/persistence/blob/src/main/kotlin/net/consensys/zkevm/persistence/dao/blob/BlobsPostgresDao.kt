package net.consensys.zkevm.persistence.dao.blob

import io.vertx.core.Future
import io.vertx.sqlclient.Row
import io.vertx.sqlclient.SqlClient
import io.vertx.sqlclient.Tuple
import kotlinx.datetime.Clock
import kotlinx.datetime.Instant
import net.consensys.decodeHex
import net.consensys.encodeHex
import net.consensys.linea.async.toSafeFuture
import net.consensys.zkevm.coordinator.clients.BlobCompressionProof
import net.consensys.zkevm.coordinator.clients.prover.serialization.BlobCompressionProofJsonResponse
import net.consensys.zkevm.domain.BlobRecord
import net.consensys.zkevm.domain.BlobStatus
import net.consensys.zkevm.persistence.db.DuplicatedRecordException
import net.consensys.zkevm.persistence.db.SQLQueryLogger
import net.consensys.zkevm.persistence.db.isDuplicateKeyException
import org.apache.logging.log4j.Level
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture

class BlobsPostgresDao(
  config: Config,
  connection: SqlClient,
  log: Logger = LogManager.getLogger(BlobsPostgresDao::class.java),
  private val clock: Clock = Clock.System
) : BlobsDao {
  private val queryLog = SQLQueryLogger(log)
  data class Config(val maxBlobsToReturn: UInt)

  companion object {
    @JvmStatic
    val TableName = "blobs"

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
        blobCompressionProof = blobCompressionProof
      )
    }

    /**
     * WARNING: Existing mappings should not change. Otherwise, can break production New One can be added
     * though.
     */
    fun blobStatusToDbValue(status: BlobStatus): Int {
      return when (status) {
        BlobStatus.COMPRESSION_PROVEN -> 1
        BlobStatus.COMPRESSION_PROVING -> 2
      }
    }

    private fun BlobCompressionProof?.toJsonString(): String? {
      return this?.let { BlobCompressionProofJsonResponse.fromDomainObject(it).toJsonString() }
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
      limit ${config.maxBlobsToReturn}
    """
      .trimIndent()

  private val selectBlobByEndBlockNumberSql =
    """
      select *
      from $TableName
      where end_block_number = $1
      limit 1
    """
      .trimIndent()

  private val selectBlobByStartBlockNumberSql =
    """
      select *
      from $TableName
      where start_block_number = $1
      limit 1
    """
      .trimIndent()

  private val insertSql =
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
  private val insertQuery = connection.preparedQuery(insertSql)
  private val deleteUptoQuery = connection.preparedQuery(deleteUptoSql)
  private val deleteAfterQuery = connection.preparedQuery(deleteAfterSql)

  override fun saveNewBlob(blobRecord: BlobRecord): SafeFuture<Unit> {
    val params: List<Any?> =
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
        blobRecord.blobCompressionProof.toJsonString()
      )
    queryLog.log(Level.TRACE, insertSql, params)

    return insertQuery.execute(Tuple.tuple(params))
      .map { }
      .recover { th ->
        if (isDuplicateKeyException(th)) {
          Future.failedFuture(
            DuplicatedRecordException("Blob ${blobRecord.intervalString()} is already persisted!", th)
          )
        } else {
          Future.failedFuture(th)
        }
      }
      .toSafeFuture()
  }

  private fun getConsecutiveBlobsFromBlockNumber(
    startingBlockNumberInclusive: ULong
  ): SafeFuture<List<BlobRecord>> {
    return selectQuery
      .execute(
        Tuple.of(
          startingBlockNumberInclusive.toLong(),
          blobStatusToDbValue(BlobStatus.COMPRESSION_PROVEN)
        )
      )
      .toSafeFuture()
      .thenApply { rowSet ->
        rowSet.map(BlobsPostgresDao::parseRecord)
      }
  }

  override fun getConsecutiveBlobsFromBlockNumber(
    startingBlockNumberInclusive: ULong,
    endBlockCreatedBefore: Instant
  ): SafeFuture<List<BlobRecord>> {
    return getConsecutiveBlobsFromBlockNumber(startingBlockNumberInclusive)
      .thenApply { blobs ->
        blobs.filter { blob ->
          blob.endBlockTime < endBlockCreatedBefore
        }
      }
  }

  override fun findBlobByStartBlockNumber(startBlockNumber: ULong): SafeFuture<BlobRecord?> {
    return selectBlobByStartBlockNumberQuery
      .execute(Tuple.of(startBlockNumber.toLong()))
      .toSafeFuture()
      .thenApply { rowSet -> rowSet.map(BlobsPostgresDao::parseRecord) }
      .thenApply { blobRecords -> blobRecords.firstOrNull() }
  }

  override fun findBlobByEndBlockNumber(
    endBlockNumber: ULong
  ): SafeFuture<BlobRecord?> {
    return selectBlobByEndBlockNumberQuery
      .execute(Tuple.of(endBlockNumber.toLong()))
      .toSafeFuture()
      .thenApply { rowSet -> rowSet.map(BlobsPostgresDao::parseRecord) }
      .thenApply { blobRecords -> blobRecords.firstOrNull() }
  }

  override fun deleteBlobsUpToEndBlockNumber(
    endBlockNumberInclusive: ULong
  ): SafeFuture<Int> {
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
}
