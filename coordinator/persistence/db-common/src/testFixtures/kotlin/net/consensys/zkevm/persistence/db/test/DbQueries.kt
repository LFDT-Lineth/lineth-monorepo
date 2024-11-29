package net.consensys.zkevm.persistence.db.test

import io.vertx.core.Future
import io.vertx.sqlclient.PreparedQuery
import io.vertx.sqlclient.Row
import io.vertx.sqlclient.RowSet
import io.vertx.sqlclient.SqlClient
import io.vertx.sqlclient.Tuple
import net.consensys.linea.async.get
import net.consensys.zkevm.domain.Batch

object DbQueries {

  const val batchesTable = "batches"
  const val blobsTable = "blobs"
  const val aggregationsTable = "aggregations"

  fun getTableContent(sqlClient: SqlClient, tableName: String): PreparedQuery<RowSet<Row>> =
    sqlClient.preparedQuery("select * from $tableName")

  private fun dbValueToStatus(value: Int): Batch.Status {
    return when (value) {
      1 -> Batch.Status.Finalized
      2 -> Batch.Status.Proven
      else ->
        throw IllegalStateException(
          "Value '$value' does not map to any ${Batch.Status::class.simpleName}"
        )
    }
  }

  fun getBatches(sqlClient: SqlClient): List<Batch> {
    return getTableContent(sqlClient, batchesTable).execute().get().map {
        record ->
      Batch(
        startBlockNumber = record.getLong("start_block_number").toULong(),
        endBlockNumber = record.getLong("end_block_number").toULong()
      )
    }
  }

  val insertBlobQuery =
    """
      insert into $blobsTable
      (created_epoch_milli, start_block_number, end_block_number,
      conflation_calculator_version, blob_hash, status, start_block_timestamp, end_block_timestamp,
      batches_count, expected_shnarf, blob_compression_proof)
      VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, CAST($11::text as jsonb))
    """.trimIndent()

  val insertBatchQueryV1 =
    """
      insert into $batchesTable
      (created_epoch_milli, start_block_number, end_block_number, prover_version, status)
      VALUES ($1, $2, $3, $4, $5)
    """.trimIndent()

  val insertBatchQueryV2 =
    """
      insert into $batchesTable
      (created_epoch_milli, start_block_number, end_block_number, prover_version, conflation_calculator_version, status)
      VALUES ($1, $2, $3, $4, $5, $6)
    """.trimIndent()

  fun insertBatch(sqlClient: SqlClient, insertQuery: String, params: List<Any>): Future<RowSet<Row>> {
    val insertBatchStatement = sqlClient.preparedQuery(insertQuery)
    return insertBatchStatement.execute(Tuple.tuple(params))
  }

  fun insertBlob(sqlClient: SqlClient, insertBlobQuery: String, params: List<Any>): Future<RowSet<Row>> {
    val insertBlobStatement = sqlClient.preparedQuery(insertBlobQuery)
    return insertBlobStatement.execute(Tuple.tuple(params))
  }
}
