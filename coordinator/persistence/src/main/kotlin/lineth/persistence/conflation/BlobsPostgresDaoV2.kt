package lineth.persistence.conflation

import io.vertx.sqlclient.Row
import io.vertx.sqlclient.SqlClient
import io.vertx.sqlclient.Tuple
import linea.domain.BlobRecordV2
import linea.domain.BlobStatus
import linea.kotlin.decodeHex
import linea.kotlin.encodeHex
import net.consensys.linea.async.toSafeFuture
import org.apache.logging.log4j.Level
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Clock
import kotlin.time.Instant

class BlobsPostgresDaoV2(
  config: Config,
  connection: SqlClient,
  log: Logger = LogManager.getLogger(BlobsPostgresDaoV2::class.java),
  clock: Clock = Clock.System,
) : BlobsPostgresDaoG<BlobRecordV2>(config.maxRollupProofsToReturn, connection, log, clock), BlobsDaoV2 {

  data class Config(val maxRollupProofsToReturn: UInt)

  // Reads block numbers only, so the rows' blobs JSON is not loaded.
  private val findHighestConsecutiveEndBlockNumberSql =
    """
      with previous_ends as (select start_block_number, end_block_number,
          lag(end_block_number, 1) over (order by start_block_number asc) as previous_end_block_number
          from $TableName
          where start_block_number >= $1 and status = $2),
        first_gapped_rollup as (select min(start_block_number) as start_block_number
          from previous_ends
          where start_block_number > $1 and start_block_number - 1 != previous_end_block_number)
      select max(end_block_number) as end_block_number
      from previous_ends
      where EXISTS (select 1 from previous_ends where start_block_number = $1)
        and ((select start_block_number from first_gapped_rollup) is null
          or start_block_number < (select start_block_number from first_gapped_rollup))
    """
      .trimIndent()

  private val findHighestConsecutiveEndBlockNumberQuery = connection.preparedQuery(
    findHighestConsecutiveEndBlockNumberSql,
  )

  companion object {
    fun parseRecord(row: Row): BlobRecordV2 {
      val blobsInfo = row.getJsonObject("blob_compression_proof")
        ?.let { BlobsInfo.fromJsonString(it.encode()) }
        ?: error(
          "blob_compression_proof is null for V2 blob at blocks " +
            "${row.getLong("start_block_number")}..${row.getLong("end_block_number")}",
        )

      return BlobRecordV2(
        startBlockNumber = row.getLong("start_block_number").toULong(),
        endBlockNumber = row.getLong("end_block_number").toULong(),
        startBlockTimestamp = Instant.fromEpochMilliseconds(row.getLong("start_block_timestamp")),
        endBlockTimestamp = Instant.fromEpochMilliseconds(row.getLong("end_block_timestamp")),
        totalBatchesCount = row.getInteger("batches_count").toUInt(),
        parentDataRollingHash = blobsInfo.parentDataRollingHash,
        dataRollingHash = blobsInfo.dataRollingHash,
        blobsData = blobsInfo.blobsData,
        proofHash = row.getString("blob_hash").decodeHex(),
        parentDataTailTakeBytes = blobsInfo.parentDataTailTakeBytes,
        boundaryPrevDataRollingHash = blobsInfo.boundaryPrevDataRollingHash,
        finalDataTailDiscardBytes = blobsInfo.finalDataTailDiscardBytes,
      )
    }
  }

  override fun parseRecord(row: Row): BlobRecordV2 = Companion.parseRecord(row)

  override fun findHighestConsecutiveEndBlockNumberFromBlockNumber(
    startingBlockNumberInclusive: ULong,
  ): SafeFuture<ULong?> {
    val params = listOf(startingBlockNumberInclusive.toLong(), blobStatusToDbValue(BlobStatus.COMPRESSION_PROVEN))
    queryLog.log(Level.TRACE, findHighestConsecutiveEndBlockNumberSql, params)
    return findHighestConsecutiveEndBlockNumberQuery
      .execute(Tuple.tuple(params))
      .toSafeFuture()
      .thenApply { rowSet -> rowSet.firstOrNull()?.getLong("end_block_number")?.toULong() }
  }

  override fun endBlockTime(record: BlobRecordV2): Instant = record.endBlockTimestamp

  override fun buildInsertParams(blobRecord: BlobRecordV2): List<Any?> =
    listOf(
      clock.now().toEpochMilliseconds(),
      blobRecord.startBlockNumber.toLong(),
      blobRecord.endBlockNumber.toLong(),
      blobRecord.proofHash.encodeHex(),
      blobStatusToDbValue(BlobStatus.COMPRESSION_PROVEN),
      blobRecord.startBlockTimestamp.toEpochMilliseconds(),
      blobRecord.endBlockTimestamp.toEpochMilliseconds(),
      blobRecord.totalBatchesCount.toInt(),
      ByteArray(0).encodeHex(),
      BlobsInfo.fromDomainObject(blobRecord).toJsonString(),
    )
}
