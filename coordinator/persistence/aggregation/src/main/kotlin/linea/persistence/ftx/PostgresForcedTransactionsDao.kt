package linea.persistence.ftx

import io.vertx.core.Future
import io.vertx.sqlclient.Row
import io.vertx.sqlclient.SqlClient
import io.vertx.sqlclient.Tuple
import net.consensys.linea.async.toSafeFuture
import net.consensys.zkevm.domain.ForcedTransactionRecord
import net.consensys.zkevm.persistence.db.PersistenceRetryer
import net.consensys.zkevm.persistence.db.SQLQueryLogger
import org.apache.logging.log4j.Level
import org.apache.logging.log4j.LogManager
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Clock
import kotlin.time.Instant

class PostgresForcedTransactionsDao(
  connection: SqlClient,
  private val clock: Clock = Clock.System,
) : ForcedTransactionsDao {
  private val log = LogManager.getLogger(this.javaClass.name)
  private val queryLog = SQLQueryLogger(log)

  companion object {
    /**
     * WARNING: Existing mappings should not change. Otherwise, can break production, new ones can be added though.
     */
    fun inclusionResultToDbValue(result: linea.forcedtx.ForcedTransactionInclusionResult): Int {
      return when (result) {
        linea.forcedtx.ForcedTransactionInclusionResult.Included -> 1
        linea.forcedtx.ForcedTransactionInclusionResult.BadNonce -> 2
        linea.forcedtx.ForcedTransactionInclusionResult.BadBalance -> 3
        linea.forcedtx.ForcedTransactionInclusionResult.BadPrecompile -> 4
        linea.forcedtx.ForcedTransactionInclusionResult.TooManyLogs -> 5
        linea.forcedtx.ForcedTransactionInclusionResult.FilteredAddressFrom -> 6
        linea.forcedtx.ForcedTransactionInclusionResult.FilteredAddressTo -> 7
        linea.forcedtx.ForcedTransactionInclusionResult.Phylax -> 8
      }
    }

    fun dbValueToInclusionResult(value: Short): linea.forcedtx.ForcedTransactionInclusionResult {
      return when (value.toInt()) {
        1 -> linea.forcedtx.ForcedTransactionInclusionResult.Included
        2 -> linea.forcedtx.ForcedTransactionInclusionResult.BadNonce
        3 -> linea.forcedtx.ForcedTransactionInclusionResult.BadBalance
        4 -> linea.forcedtx.ForcedTransactionInclusionResult.BadPrecompile
        5 -> linea.forcedtx.ForcedTransactionInclusionResult.TooManyLogs
        6 -> linea.forcedtx.ForcedTransactionInclusionResult.FilteredAddressFrom
        7 -> linea.forcedtx.ForcedTransactionInclusionResult.FilteredAddressTo
        8 -> linea.forcedtx.ForcedTransactionInclusionResult.Phylax
        else -> throw IllegalArgumentException("Unknown inclusion_result value: $value")
      }
    }

    fun proofStatusToDbValue(status: ForcedTransactionRecord.ProofStatus): Int {
      return when (status) {
        ForcedTransactionRecord.ProofStatus.UNREQUESTED -> 1
        ForcedTransactionRecord.ProofStatus.REQUESTED -> 2
        ForcedTransactionRecord.ProofStatus.PROVEN -> 3
      }
    }

    fun dbValueToProofStatus(value: Short): ForcedTransactionRecord.ProofStatus {
      return when (value.toInt()) {
        1 -> ForcedTransactionRecord.ProofStatus.UNREQUESTED
        2 -> ForcedTransactionRecord.ProofStatus.REQUESTED
        3 -> ForcedTransactionRecord.ProofStatus.PROVEN
        else -> throw IllegalArgumentException("Unknown proof_status value: $value")
      }
    }

    private fun parseRecord(row: Row): ForcedTransactionRecord {
      val ftxNumber = row.getLong("ftx_number").toULong()
      val inclusionResult = dbValueToInclusionResult(row.getShort("inclusion_result"))
      val simulatedExecutionBlockNumber = row.getLong("simulated_execution_block_number").toULong()
      val simulatedExecutionBlockTimestamp = Instant.fromEpochMilliseconds(
        row.getLong("simulated_execution_block_timestamp"),
      )
      val proofStatus = dbValueToProofStatus(row.getShort("proof_status"))

      return ForcedTransactionRecord(
        ftxNumber = ftxNumber,
        inclusionResult = inclusionResult,
        simulatedExecutionBlockNumber = simulatedExecutionBlockNumber,
        simulatedExecutionBlockTimestamp = simulatedExecutionBlockTimestamp,
        proofStatus = proofStatus,
        proofIndex = null,
      )
    }
  }

  private val upsertSql =
    """
      insert into forced_transactions
      (created_epoch_milli, updated_epoch_milli, ftx_number, inclusion_result,
       simulated_execution_block_number, simulated_execution_block_timestamp, proof_status)
      VALUES ($1, $2, $3, $4, $5, $6, $7)
      ON CONFLICT (ftx_number)
      DO UPDATE SET
        updated_epoch_milli = EXCLUDED.updated_epoch_milli,
        inclusion_result = EXCLUDED.inclusion_result,
        simulated_execution_block_number = EXCLUDED.simulated_execution_block_number,
        simulated_execution_block_timestamp = EXCLUDED.simulated_execution_block_timestamp,
        proof_status = EXCLUDED.proof_status
    """.trimIndent()

  private val upsertQuery = connection.preparedQuery(upsertSql)

  private val selectByNumberSql =
    """
      select ftx_number, inclusion_result, simulated_execution_block_number,
             simulated_execution_block_timestamp, proof_status
      from forced_transactions
      where ftx_number = $1
    """.trimIndent()

  private val selectByNumberQuery = connection.preparedQuery(selectByNumberSql)

  private val selectAllSql =
    """
      select ftx_number, inclusion_result, simulated_execution_block_number,
             simulated_execution_block_timestamp, proof_status
      from forced_transactions
      order by ftx_number
    """.trimIndent()

  private val selectAllQuery = connection.preparedQuery(selectAllSql)

  private val deleteSql =
    """
      delete from forced_transactions
      where ftx_number <= $1
    """.trimIndent()

  private val deleteQuery = connection.preparedQuery(deleteSql)

  override fun save(ftx: ForcedTransactionRecord): SafeFuture<Unit> {
    val now = clock.now().toEpochMilliseconds()
    val params: List<Any?> = listOf(
      now,
      now,
      ftx.ftxNumber.toLong(),
      inclusionResultToDbValue(ftx.inclusionResult),
      ftx.simulatedExecutionBlockNumber.toLong(),
      ftx.simulatedExecutionBlockTimestamp.toEpochMilliseconds(),
      proofStatusToDbValue(ftx.proofStatus),
    )
    queryLog.log(Level.TRACE, upsertSql, params)
    return upsertQuery.execute(Tuple.tuple(params))
      .map { }
      .recover { th -> Future.failedFuture(th) }
      .toSafeFuture()
  }

  override fun findByNumber(ftxNumber: ULong): SafeFuture<ForcedTransactionRecord?> {
    val params = listOf(ftxNumber.toLong())
    queryLog.log(Level.TRACE, selectByNumberSql, params)
    return selectByNumberQuery.execute(Tuple.tuple(params))
      .map { rowSet -> rowSet.firstOrNull()?.let(::parseRecord) }
      .recover { th -> Future.failedFuture(th) }
      .toSafeFuture()
  }

  override fun list(): SafeFuture<List<ForcedTransactionRecord>> {
    queryLog.log(Level.TRACE, selectAllSql)
    return selectAllQuery.execute()
      .map { rowSet -> rowSet.map(::parseRecord) }
      .recover { th -> Future.failedFuture(th) }
      .toSafeFuture()
  }

  override fun deleteFtxUpToInclusive(ftxNumber: ULong): SafeFuture<Int> {
    val params = listOf(ftxNumber.toLong())
    queryLog.log(Level.TRACE, deleteSql, params)
    return deleteQuery.execute(Tuple.tuple(params))
      .map { rowSet -> rowSet.rowCount() }
      .recover { th -> Future.failedFuture(th) }
      .toSafeFuture()
  }
}

class RetryingPostgresForcedTransactionsDao(
  private val delegate: PostgresForcedTransactionsDao,
  private val persistenceRetryer: PersistenceRetryer,
) : ForcedTransactionsDao {
  override fun save(ftx: ForcedTransactionRecord): SafeFuture<Unit> {
    return persistenceRetryer.retryQuery({ delegate.save(ftx) })
  }

  override fun findByNumber(ftxNumber: ULong): SafeFuture<ForcedTransactionRecord?> {
    return persistenceRetryer.retryQuery({ delegate.findByNumber(ftxNumber) })
  }

  override fun list(): SafeFuture<List<ForcedTransactionRecord>> {
    return persistenceRetryer.retryQuery({ delegate.list() })
  }

  override fun deleteFtxUpToInclusive(ftxNumber: ULong): SafeFuture<Int> {
    return persistenceRetryer.retryQuery({ delegate.deleteFtxUpToInclusive(ftxNumber) })
  }
}
