package lineth.persistence.conflation

import io.vertx.sqlclient.Row
import io.vertx.sqlclient.SqlClient
import linea.domain.BlobRecordV2
import linea.domain.BlobStatus
import linea.kotlin.encodeHex
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import kotlin.time.Clock
import kotlin.time.Instant

class BlobsPostgresDaoV2(
  config: Config,
  connection: SqlClient,
  log: Logger = LogManager.getLogger(BlobsPostgresDaoV2::class.java),
  clock: Clock = Clock.System,
) : BlobsPostgresDaoG<BlobRecordV2>(config.maxRollupProofsToReturn, connection, log, clock), BlobsDaoV2 {

  data class Config(val maxRollupProofsToReturn: UInt)

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
        parentShnarf = blobsInfo.parentShnarf,
        endShnarf = blobsInfo.endShnarf,
        blobsData = blobsInfo.blobsData,
      )
    }
  }

  override fun parseRecord(row: Row): BlobRecordV2 = Companion.parseRecord(row)

  override fun endBlockTime(record: BlobRecordV2): Instant = record.endBlockTimestamp

  override fun buildInsertParams(blobRecord: BlobRecordV2): List<Any?> =
    listOf(
      clock.now().toEpochMilliseconds(),
      blobRecord.startBlockNumber.toLong(),
      blobRecord.endBlockNumber.toLong(),
      ByteArray(0).encodeHex(),
      blobStatusToDbValue(BlobStatus.COMPRESSION_PROVEN),
      blobRecord.startBlockTimestamp.toEpochMilliseconds(),
      blobRecord.endBlockTimestamp.toEpochMilliseconds(),
      blobRecord.totalBatchesCount.toInt(),
      ByteArray(0).encodeHex(),
      BlobsInfo.fromDomainObject(blobRecord).toJsonString(),
    )
}
