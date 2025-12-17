package net.consensys.zkevm.persistence.dao.feehistory

import io.vertx.core.Future
import io.vertx.sqlclient.SqlClient
import io.vertx.sqlclient.Tuple
import kotlinx.datetime.Clock
import linea.domain.FeeHistory
import net.consensys.linea.async.toSafeFuture
import net.consensys.zkevm.persistence.db.SQLQueryLogger
import org.apache.logging.log4j.Level
import org.apache.logging.log4j.LogManager
import tech.pegasys.teku.infrastructure.async.SafeFuture

interface FeeHistoriesDao {
  fun saveNewFeeHistory(feeHistory: FeeHistory, rewardPercentiles: List<Double>): SafeFuture<Unit>

  fun findBaseFeePerGasAtPercentile(percentile: Double, fromBlockNumber: Long): SafeFuture<ULong?>

  fun findBaseFeePerBlobGasAtPercentile(percentile: Double, fromBlockNumber: Long): SafeFuture<ULong?>

  fun findAverageRewardAtPercentile(rewardPercentile: Double, fromBlockNumber: Long): SafeFuture<ULong?>

  fun findHighestBlockNumberWithPercentile(rewardPercentile: Double): SafeFuture<Long?>

  fun getNumOfFeeHistoriesFromBlockNumber(rewardPercentile: Double, fromBlockNumber: Long): SafeFuture<Int>

  fun deleteFeeHistoriesUpToBlockNumber(blockNumberInclusive: Long): SafeFuture<Int>
}

class FeeHistoriesPostgresDao(
  connection: SqlClient,
  private val clock: Clock = Clock.System,
) : FeeHistoriesDao {
  private val log = LogManager.getLogger(this.javaClass.name)
  private val queryLog = SQLQueryLogger(log)

  companion object {
    @JvmStatic
    val TableName = "feehistories"
  }

  private val upsertSql =
    """
     insert into $TableName
     (created_epoch_milli, block_number, base_fee_per_gas,
     base_fee_per_blob_gas, gas_used_ratio, blob_gas_used_ratio, rewards, reward_percentiles)
     VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
     ON CONFLICT ON CONSTRAINT ${TableName}_pkey
     DO UPDATE SET
      created_epoch_milli = EXCLUDED.created_epoch_milli,
      base_fee_per_gas = EXCLUDED.base_fee_per_gas,
      base_fee_per_blob_gas = EXCLUDED.base_fee_per_blob_gas,
      gas_used_ratio = EXCLUDED.gas_used_ratio,
      blob_gas_used_ratio = EXCLUDED.blob_gas_used_ratio,
      reward_percentiles = ARRAY(SELECT unnest($TableName.reward_percentiles || EXCLUDED.reward_percentiles) as x order by x),
      rewards = ARRAY(SELECT unnest($TableName.rewards || EXCLUDED.rewards) as x order by x);
    """
      .trimIndent()

  private val selectHighestBlockNumberSql =
    """
      select max(block_number) as highest_block_number from $TableName
      where $1 = ANY(reward_percentiles)
    """
      .trimIndent()

  private val getNthPercentileOfBaseFeePerGasSql =
    """
      select percentile_cont($1) within group (order by base_fee_per_gas) as percentile_value from $TableName
      where block_number >= $2
    """
      .trimIndent()

  private val getNthPercentileOfBaseFeePerBlobGasSql =
    """
      select percentile_cont($1) within group (order by base_fee_per_blob_gas) as percentile_value from $TableName
      where block_number >= $2
    """
      .trimIndent()

  private val getAvgNthPercentileRewardSql =
    """
      select avg(fee_histories.rewards[p.idx]) as avg_reward from $TableName fee_histories
      cross join lateral unnest(reward_percentiles) with ordinality as p(percentile, idx)
      where block_number >= $2 and p.percentile = $1
    """
      .trimIndent()

  private val countFeeHistoriesFromBlockNumberSql =
    """
      select count(*) as fee_history_count from $TableName
      where block_number >= $2 and $1 = ANY(reward_percentiles)
    """
      .trimIndent()

  private val deleteSql =
    """
      delete from $TableName
      where block_number <= $1
    """
      .trimIndent()

  private val upsertQuery = connection.preparedQuery(upsertSql)
  private val selectHighestBlockNumberQuery = connection.preparedQuery(selectHighestBlockNumberSql)
  private val getNthPercentileOfBaseFeePerGasQuery = connection.preparedQuery(getNthPercentileOfBaseFeePerGasSql)
  private val getNthPercentileOfBaseFeePerBlobGasQuery = connection.preparedQuery(
    getNthPercentileOfBaseFeePerBlobGasSql,
  )
  private val getAvgNthPercentileRewardQuery = connection.preparedQuery(getAvgNthPercentileRewardSql)
  private val countFeeHistoriesFromBlockNumberQuery = connection.preparedQuery(countFeeHistoriesFromBlockNumberSql)
  private val deleteQuery = connection.preparedQuery(deleteSql)

  override fun saveNewFeeHistory(feeHistory: FeeHistory, rewardPercentiles: List<Double>): SafeFuture<Unit> {
    val batch = feeHistory.reward.mapIndexed { i, reward ->
      val params = listOf(
        clock.now().toEpochMilliseconds(),
        feeHistory.oldestBlock.toLong() + i,
        feeHistory.baseFeePerGas[i].toLong(),
        (feeHistory.baseFeePerBlobGas.getOrElse(i) { 0uL }).toLong(),
        feeHistory.gasUsedRatio[i].toFloat(),
        feeHistory.blobGasUsedRatio.getOrElse(i) { 0.0 }.toFloat(),
        reward.map { it.toLong() }.toTypedArray(),
        rewardPercentiles.map { it.toFloat() }.toTypedArray(),
      )
      queryLog.log(Level.TRACE, upsertSql, params)
      Tuple.tuple(params)
    }
    return upsertQuery.executeBatch(batch)
      .map { }
      .recover { th ->
        Future.failedFuture(th)
      }
      .toSafeFuture()
  }

  override fun findBaseFeePerGasAtPercentile(percentile: Double, fromBlockNumber: Long): SafeFuture<ULong?> {
    val params = listOf(
      percentile.div(100).toFloat(),
      fromBlockNumber,
    )
    queryLog.log(Level.TRACE, getNthPercentileOfBaseFeePerGasSql, params)
    return getNthPercentileOfBaseFeePerGasQuery
      .execute(Tuple.tuple(params))
      .toSafeFuture()
      .thenApply { rowSet ->
        rowSet.firstOrNull()?.getDouble("percentile_value")?.toULong()
      }
  }

  override fun findBaseFeePerBlobGasAtPercentile(percentile: Double, fromBlockNumber: Long): SafeFuture<ULong?> {
    val params = listOf(
      percentile.div(100).toFloat(),
      fromBlockNumber,
    )
    queryLog.log(Level.TRACE, getNthPercentileOfBaseFeePerBlobGasSql, params)
    return getNthPercentileOfBaseFeePerBlobGasQuery
      .execute(Tuple.tuple(params))
      .toSafeFuture()
      .thenApply { rowSet ->
        rowSet.firstOrNull()?.getDouble("percentile_value")?.toULong()
      }
  }

  override fun findAverageRewardAtPercentile(rewardPercentile: Double, fromBlockNumber: Long): SafeFuture<ULong?> {
    val params = listOf(
      rewardPercentile.toFloat(),
      fromBlockNumber,
    )
    queryLog.log(Level.TRACE, getAvgNthPercentileRewardSql, params)
    return getAvgNthPercentileRewardQuery
      .execute(Tuple.tuple(params))
      .toSafeFuture()
      .thenApply { rowSet ->
        rowSet.firstOrNull()?.getDouble("avg_reward")?.toULong()
      }
  }

  override fun findHighestBlockNumberWithPercentile(rewardPercentile: Double): SafeFuture<Long?> {
    val params = listOf(rewardPercentile.toFloat())
    queryLog.log(Level.TRACE, selectHighestBlockNumberSql, params)
    return selectHighestBlockNumberQuery
      .execute(Tuple.tuple(params))
      .toSafeFuture()
      .thenApply { rowSet ->
        rowSet.firstOrNull()?.getLong("highest_block_number")
      }
  }

  override fun getNumOfFeeHistoriesFromBlockNumber(rewardPercentile: Double, fromBlockNumber: Long): SafeFuture<Int> {
    val params = listOf(
      rewardPercentile.toFloat(),
      fromBlockNumber,
    )
    queryLog.log(Level.TRACE, countFeeHistoriesFromBlockNumberSql, params)
    return countFeeHistoriesFromBlockNumberQuery
      .execute(Tuple.tuple(params))
      .toSafeFuture()
      .thenApply { rowSet ->
        rowSet.firstOrNull()?.getLong("fee_history_count")?.toInt() ?: 0
      }
  }

  override fun deleteFeeHistoriesUpToBlockNumber(blockNumberInclusive: Long): SafeFuture<Int> {
    return deleteQuery
      .execute(Tuple.of(blockNumberInclusive))
      .map { rowSet -> rowSet.rowCount() }
      .toSafeFuture()
  }
}
