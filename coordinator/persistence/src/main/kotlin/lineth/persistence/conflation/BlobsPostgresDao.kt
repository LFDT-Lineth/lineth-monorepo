package lineth.persistence.conflation

import io.vertx.sqlclient.Row
import io.vertx.sqlclient.SqlClient
import linea.domain.BlobCompressionProof
import linea.domain.BlobRecord
import linea.domain.BlobStatus
import linea.kotlin.decodeHex
import linea.kotlin.encodeHex
import lineth.coordinator.clients.prover.serialization.BlobCompressionProofJsonResponse
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import kotlin.time.Clock
import kotlin.time.Instant

class BlobsPostgresDao(
  config: Config,
  connection: SqlClient,
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

  override fun endBlockTime(record: BlobRecord): Instant = record.endBlockTime

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
}
