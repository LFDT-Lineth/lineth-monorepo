package net.consensys.zkevm.persistence.dao.rejectedtransaction

import com.fasterxml.jackson.databind.ObjectMapper
import io.vertx.core.Future
import io.vertx.sqlclient.Row
import io.vertx.sqlclient.SqlClient
import io.vertx.sqlclient.Tuple
import kotlinx.datetime.Clock
import kotlinx.datetime.Instant
import net.consensys.encodeHex
import net.consensys.linea.async.toSafeFuture
import net.consensys.linea.transactionexclusion.ModuleOverflow
import net.consensys.linea.transactionexclusion.RejectedTransaction
import net.consensys.linea.transactionexclusion.TransactionInfo
import net.consensys.zkevm.persistence.db.DuplicatedRecordException
import net.consensys.zkevm.persistence.db.SQLQueryLogger
import net.consensys.zkevm.persistence.db.isDuplicateKeyException
import org.apache.logging.log4j.Level
import org.apache.logging.log4j.LogManager
import tech.pegasys.teku.infrastructure.async.SafeFuture

class RejectedTransactionsPostgresDao(
  private val readConnection: SqlClient,
  private val writeConnection: SqlClient,
  private val clock: Clock = Clock.System
) : RejectedTransactionsDao {
  private val log = LogManager.getLogger(this.javaClass.name)
  private val queryLog = SQLQueryLogger(log)

  companion object {
    // Public instead of internal to allow usage in integrationTest source set
    fun rejectedStageToDbValue(txRejectionStage: RejectedTransaction.Stage): String {
      return when (txRejectionStage) {
        RejectedTransaction.Stage.SEQUENCER -> "SEQ"
        RejectedTransaction.Stage.RPC -> "RPC"
        RejectedTransaction.Stage.P2P -> "P2P"
      }
    }

    fun dbValueToRejectedStage(dbStrValue: String): RejectedTransaction.Stage {
      return when (dbStrValue) {
        "SEQ" -> RejectedTransaction.Stage.SEQUENCER
        "RPC" -> RejectedTransaction.Stage.RPC
        "P2P" -> RejectedTransaction.Stage.P2P
        else -> throw IllegalStateException(
          "The db string value: \"$dbStrValue\" cannot be mapped to any RejectedTransaction.Stage enums: " +
            RejectedTransaction.Stage.entries.joinToString(",", "[", "]") { it.name }
        )
      }
    }

    fun parseModuleOverflowListFromJsonString(jsonString: String): List<ModuleOverflow> {
      return ObjectMapper().readValue(
        jsonString,
        Array<ModuleOverflow>::class.java
      ).toList()
    }

    fun parseRecord(record: Row): RejectedTransaction {
      return RejectedTransaction(
        txRejectionStage = record.getString("reject_stage").run(::dbValueToRejectedStage),
        timestamp = Instant.fromEpochMilliseconds(record.getLong("reject_timestamp")),
        blockNumber = record.getLong("block_number")?.toULong(),
        transactionRLP = record.getBuffer("tx_rlp").bytes,
        reasonMessage = record.getString("reject_reason"),
        overflows = parseModuleOverflowListFromJsonString(
          record.getJsonArray("overflows").encode()
        ),
        transactionInfo = TransactionInfo(
          hash = record.getBuffer("tx_hash").bytes,
          from = record.getBuffer("tx_from").bytes,
          to = record.getBuffer("tx_to")?.bytes,
          nonce = record.getLong("tx_nonce").toULong()
        )
      )
    }

    @JvmStatic
    val rejectedTransactionsTable = "rejected_transactions"

    @JvmStatic
    val fullTransactionsTable = "full_transactions"
  }

  private val insertSql =
    """
      with x as (
        insert into $rejectedTransactionsTable
        (created_epoch_milli, tx_hash, tx_from, tx_to, tx_nonce,
        reject_stage, reject_reason, reject_timestamp, block_number, overflows)
        values ($1, $2, $3, $4, $5, $6, $7, $8, $9, cast($10::text as jsonb))
        returning tx_hash
      )
      insert into $fullTransactionsTable
      (tx_hash, tx_rlp)
      select x.tx_hash, $11
      from x
      on conflict on constraint ${fullTransactionsTable}_pkey
      do nothing
    """
      .trimIndent()

  private val selectSql =
    """
      select
        $rejectedTransactionsTable.*,
        $fullTransactionsTable.tx_rlp as tx_rlp
      from $rejectedTransactionsTable
      join $fullTransactionsTable on $rejectedTransactionsTable.tx_hash = $fullTransactionsTable.tx_hash
      where $fullTransactionsTable.tx_hash = $1 and $rejectedTransactionsTable.reject_timestamp >= $2
      order by $rejectedTransactionsTable.reject_timestamp desc
      limit 1
    """
      .trimIndent()

  private val deleteRejectedTransactionsSql =
    """
      delete from $rejectedTransactionsTable
      where created_epoch_milli < $1
    """
      .trimIndent()

  private val deleteFullTransactionsSql =
    """
      delete from $fullTransactionsTable
      f where not exists (select null from $rejectedTransactionsTable x where f.tx_hash = x.tx_hash)
    """
      .trimIndent()

  private val insertSqlQuery = writeConnection.preparedQuery(insertSql)
  private val selectSqlQuery = readConnection.preparedQuery(selectSql)
  private val deleteRejectedTransactionsSqlQuery = writeConnection.preparedQuery(deleteRejectedTransactionsSql)
  private val deleteFullTransactionsSqlQuery = writeConnection.preparedQuery(deleteFullTransactionsSql)

  override fun saveNewRejectedTransaction(rejectedTransaction: RejectedTransaction): SafeFuture<Unit> {
    val params: List<Any?> =
      listOf(
        clock.now().toEpochMilliseconds(),
        rejectedTransaction.transactionInfo.hash,
        rejectedTransaction.transactionInfo.from,
        rejectedTransaction.transactionInfo.to,
        rejectedTransaction.transactionInfo.nonce.toLong(),
        rejectedStageToDbValue(rejectedTransaction.txRejectionStage),
        rejectedTransaction.reasonMessage,
        rejectedTransaction.timestamp.toEpochMilliseconds(),
        rejectedTransaction.blockNumber?.toLong(),
        ObjectMapper().writeValueAsString(rejectedTransaction.overflows),
        rejectedTransaction.transactionRLP
      )
    queryLog.log(Level.TRACE, insertSql, params)

    return insertSqlQuery.execute(Tuple.tuple(params))
      .map { }
      .recover { th ->
        if (isDuplicateKeyException(th)) {
          Future.failedFuture(
            DuplicatedRecordException(
              "RejectedTransaction ${rejectedTransaction.transactionInfo.hash.encodeHex()} is already persisted!",
              th
            )
          )
        } else {
          Future.failedFuture(th)
        }
      }
      .toSafeFuture()
  }

  override fun findRejectedTransactionByTxHash(
    txHash: ByteArray,
    notRejectedBefore: Instant
  ): SafeFuture<RejectedTransaction?> {
    return selectSqlQuery
      .execute(
        Tuple.of(
          txHash,
          notRejectedBefore.toEpochMilliseconds()
        )
      )
      .toSafeFuture()
      .thenApply { rowSet -> rowSet.map(::parseRecord) }
      .thenApply { rejectedTxRecords -> rejectedTxRecords.firstOrNull() }
  }

  override fun deleteRejectedTransactions(
    createdBefore: Instant
  ): SafeFuture<Int> {
    return deleteRejectedTransactionsSqlQuery
      .execute(Tuple.of(createdBefore.toEpochMilliseconds()))
      .map { rowSet -> rowSet.rowCount() }
      .also {
        deleteFullTransactionsSqlQuery.execute()
      }
      .toSafeFuture()
  }
}
