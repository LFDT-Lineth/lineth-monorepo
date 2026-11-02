package lineth.persistence.conflation

import io.vertx.sqlclient.Row
import io.vertx.sqlclient.SqlClient
import io.vertx.sqlclient.Tuple
import linea.domain.AggregationG
import linea.domain.BlobAndBatchCounters
import linea.domain.BlobCounters
import linea.domain.BlockIntervals
import linea.domain.ProofToFinalize
import linea.kotlin.decodeHex
import lineth.coordinator.clients.prover.serialization.ProofToFinalizeJsonResponse
import lineth.persistence.AggregationsDao
import net.consensys.linea.async.toSafeFuture
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Clock
import kotlin.time.Instant

class PostgresAggregationsDao(
  connection: SqlClient,
  clock: Clock = Clock.System,
) : PostgresAggregationsDaoG<ProofToFinalize>(connection, clock), AggregationsDao {

  private val selectBatchesAndBlobsForAggregation = connection.preparedQuery(
    """
      with blob_previous_ends as (
        select *,
          coalesce(max(end_block_number) over (order by start_block_number asc, end_block_number asc ROWS BETWEEN UNBOUNDED PRECEDING AND 1 PRECEDING), 0) as max_blob_end
        from $blobsTable
        where start_block_number >= $1 -- EXPECTED_START_BLOCK_NUMBER
          AND status = 1
        order by start_block_number asc
      ),
      removed_old_blobs as (
        select *
        from blob_previous_ends
        where end_block_number > max_blob_end
      ),
      first_gapped_blob as (
        select start_block_number
        from removed_old_blobs
        where start_block_number > $1 and start_block_number - 1 != max_blob_end
        limit 1
      ),
      consecutive_proven_blobs as (
        select *
        from removed_old_blobs
        where EXISTS (select 1 from blobs where start_block_number = $1 and status = 1)
          and (removed_old_blobs.max_blob_end = removed_old_blobs.start_block_number - 1 or removed_old_blobs.start_block_number = $1)
          and ((select count(1) from first_gapped_blob) = 0 or removed_old_blobs.start_block_number < (select * from first_gapped_blob))
      ),
      batches_previous_ends as (
        select *,
          coalesce(max(end_block_number) over (order by start_block_number asc, end_block_number asc ROWS BETWEEN UNBOUNDED PRECEDING AND 1 PRECEDING), 0) as max_batch_end
        from $batchesTable
        where status = 2
          AND start_block_number >= $1 -- EXPECTED_START_BLOCK_NUMBER
        order by start_block_number asc
      ),
      removed_old_batches as (
        select *
        from batches_previous_ends
        where end_block_number > max_batch_end
      ),
      first_gapped_batch as (
        select start_block_number
        from removed_old_batches
        where start_block_number > $1 and start_block_number - 1 != max_batch_end
        limit 1
      ),
      consecutive_proven_batches as (
        select *
        from removed_old_batches
        where EXISTS (select 1 from batches where start_block_number = $1 and status = 2)
          and (removed_old_batches.max_batch_end = removed_old_batches.start_block_number - 1 or removed_old_batches.start_block_number = $1)
          and ((select count(1) from first_gapped_batch) = 0 or removed_old_batches.start_block_number < (select * from first_gapped_batch))
      ),
      final_blobs as (
        select
          bl.start_block_number as blob_start_block_number, bl.end_block_number as blob_end_block_number, bl.batches_count, bl.start_block_timestamp, bl.end_block_timestamp, bl.expected_shnarf as blob_expected_shnarf,
          ba.start_block_number, ba.end_block_number
        from consecutive_proven_blobs as bl
        LEFT JOIN consecutive_proven_batches as ba
        ON ba.start_block_number >= bl.start_block_number
          AND ba.end_block_number <= bl.end_block_number
        where ba.start_block_number is not null
        ORDER BY bl.start_block_number ASC
      )
      select * from final_blobs
    """.trimIndent(),
  )

  override fun serializeProof(proof: ProofToFinalize?): String? {
    return proof?.let { ProofToFinalizeJsonResponse.fromDomainObject(it).toJsonString() }
  }

  override fun deserializeProof(json: String): ProofToFinalize {
    return ProofToFinalizeJsonResponse.fromJsonString(json).toDomainObject()
  }

  override fun proofFinalTimestamp(proof: ProofToFinalize): Instant = proof.finalTimestamp

  override fun saveNewAggregation(aggregation: AggregationG<ProofToFinalize>): SafeFuture<Unit> {
    return saveAggregationRecord(aggregation)
  }

  override fun findConsecutiveProvenBlobs(fromBlockNumber: Long): SafeFuture<List<BlobAndBatchCounters>> {
    return selectBatchesAndBlobsForAggregation
      .execute(Tuple.of(fromBlockNumber))
      .toSafeFuture()
      .thenApply { rowSet ->
        val batches: List<BatchRecordWithBlobInfo> = rowSet.map { row -> parseBatchAndBlobRecord(row) }
        val batchesByBlob: Map<ULong, List<BatchRecordWithBlobInfo>> = batches.groupBy { it.blobStartBlockNumber }
        val result = batchesByBlob
          .map { (_, blobBatches) ->
            val firstBatch = blobBatches.first()
            val blobCounters = BlobCounters(
              numberOfBatches = firstBatch.batchesCount,
              startBlockNumber = firstBatch.blobStartBlockNumber,
              endBlockNumber = firstBatch.blobEndBlockNumber,
              startBlockTimestamp = firstBatch.blobStartBlockTimestamp,
              endBlockTimestamp = firstBatch.blobEndBlockTimestamp,
              expectedShnarf = firstBatch.blobExpectedShnarf,
            )
            BlobAndBatchCounters(
              blobCounters = blobCounters,
              executionProofs = BlockIntervals(
                startingBlockNumber = blobBatches.first().batchStartBlockNumber,
                upperBoundaries = blobBatches.map { it.batchEndBlockNumber },
              ),
            )
          }
          .filter {
            it.blobCounters.endBlockNumber == it.executionProofs.upperBoundaries.last()
          }
          .sortedBy { it.blobCounters.startBlockNumber }
        if (result.isNotEmpty() && result.first().blobCounters.startBlockNumber == fromBlockNumber.toULong()) {
          result
        } else {
          emptyList()
        }
      }
  }

  private fun parseBatchAndBlobRecord(record: Row): BatchRecordWithBlobInfo {
    return BatchRecordWithBlobInfo(
      blobStartBlockNumber = record.getLong("blob_start_block_number").toULong(),
      blobEndBlockNumber = record.getLong("blob_end_block_number").toULong(),
      blobStartBlockTimestamp = Instant.fromEpochMilliseconds(record.getLong("start_block_timestamp")),
      blobEndBlockTimestamp = Instant.fromEpochMilliseconds(record.getLong("end_block_timestamp")),
      blobExpectedShnarf = record.getString("blob_expected_shnarf").decodeHex(),
      batchesCount = record.getLong("batches_count").toUInt(),
      batchStartBlockNumber = record.getLong("start_block_number").toULong(),
      batchEndBlockNumber = record.getLong("end_block_number").toULong(),
    )
  }

  private data class BatchRecordWithBlobInfo(
    val blobStartBlockNumber: ULong,
    val blobEndBlockNumber: ULong,
    val blobStartBlockTimestamp: Instant,
    val blobEndBlockTimestamp: Instant,
    val blobExpectedShnarf: ByteArray,
    val batchesCount: UInt,
    val batchStartBlockNumber: ULong,
    val batchEndBlockNumber: ULong,
  ) {
    override fun equals(other: Any?): Boolean {
      if (this === other) return true
      if (javaClass != other?.javaClass) return false

      other as BatchRecordWithBlobInfo

      if (blobStartBlockNumber != other.blobStartBlockNumber) return false
      if (blobEndBlockNumber != other.blobEndBlockNumber) return false
      if (blobStartBlockTimestamp != other.blobStartBlockTimestamp) return false
      if (blobEndBlockTimestamp != other.blobEndBlockTimestamp) return false
      if (!blobExpectedShnarf.contentEquals(other.blobExpectedShnarf)) return false
      if (batchesCount != other.batchesCount) return false
      if (batchStartBlockNumber != other.batchStartBlockNumber) return false
      if (batchEndBlockNumber != other.batchEndBlockNumber) return false

      return true
    }

    override fun hashCode(): Int {
      var result = blobStartBlockNumber.hashCode()
      result = 31 * result + blobEndBlockNumber.hashCode()
      result = 31 * result + blobStartBlockTimestamp.hashCode()
      result = 31 * result + blobEndBlockTimestamp.hashCode()
      result = 31 * result + blobExpectedShnarf.contentHashCode()
      result = 31 * result + batchesCount.hashCode()
      result = 31 * result + batchStartBlockNumber.hashCode()
      result = 31 * result + batchEndBlockNumber.hashCode()
      return result
    }
  }
}
