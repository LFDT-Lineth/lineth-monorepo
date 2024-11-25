package net.consensys.zkevm.persistence.dao.rejectedtransaction

import com.fasterxml.jackson.databind.ObjectMapper
import io.vertx.junit5.VertxExtension
import io.vertx.sqlclient.PreparedQuery
import io.vertx.sqlclient.Row
import io.vertx.sqlclient.RowSet
import kotlinx.datetime.Clock
import kotlinx.datetime.Instant
import net.consensys.FakeFixedClock
import net.consensys.decodeHex
import net.consensys.encodeHex
import net.consensys.linea.async.get
import net.consensys.linea.transactionexclusion.ModuleOverflow
import net.consensys.linea.transactionexclusion.RejectedTransaction
import net.consensys.linea.transactionexclusion.TransactionInfo
import net.consensys.linea.transactionexclusion.test.defaultRejectedTransaction
import net.consensys.linea.transactionexclusion.test.rejectedContractDeploymentTransaction
import net.consensys.trimToMillisecondPrecision
import net.consensys.zkevm.persistence.db.DbHelper
import net.consensys.zkevm.persistence.db.DuplicatedRecordException
import net.consensys.zkevm.persistence.db.test.CleanDbTestSuiteParallel
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.assertThrows
import org.junit.jupiter.api.extension.ExtendWith
import java.util.concurrent.ExecutionException
import kotlin.time.Duration.Companion.hours
import kotlin.time.Duration.Companion.seconds

@ExtendWith(VertxExtension::class)
class RejectedTransactionsPostgresDaoTest : CleanDbTestSuiteParallel() {
  init {
    target = "3"
  }

  override val databaseName = DbHelper.generateUniqueDbName("tx-exclusion-api-rejectedtxns-dao-tests")
  private var fakeClock = FakeFixedClock(Clock.System.now())
  private lateinit var rejectedTransactionsPostgresDao: RejectedTransactionsPostgresDao
  private lateinit var notRejectedBefore: Instant

  // Helper functions
  private fun createRejectedTransaction(
    txRejectionStage: RejectedTransaction.Stage = defaultRejectedTransaction.txRejectionStage,
    timestamp: Instant = fakeClock.now().minus(10.seconds),
    blockNumber: ULong? = defaultRejectedTransaction.blockNumber,
    transactionRLP: ByteArray = defaultRejectedTransaction.transactionRLP,
    reasonMessage: String = defaultRejectedTransaction.reasonMessage,
    overflows: List<ModuleOverflow> = defaultRejectedTransaction.overflows,
    transactionInfo: TransactionInfo = defaultRejectedTransaction.transactionInfo
  ): RejectedTransaction {
    return RejectedTransaction(
      txRejectionStage = txRejectionStage,
      timestamp = timestamp.trimToMillisecondPrecision(),
      blockNumber = blockNumber,
      transactionRLP = transactionRLP,
      reasonMessage = reasonMessage,
      overflows = overflows,
      transactionInfo = transactionInfo
    )
  }

  private fun dbTableContentQuery(dbTableName: String): PreparedQuery<RowSet<Row>> =
    sqlClient.preparedQuery("select * from $dbTableName")

  private fun rejectedTransactionsTotalRows(): Int =
    dbTableContentQuery(RejectedTransactionsPostgresDao.rejectedTransactionsTable).execute().get().size()

  private fun fullTransactionsTotalRows(): Int =
    dbTableContentQuery(RejectedTransactionsPostgresDao.fullTransactionsTable).execute().get().size()

  @BeforeEach
  fun beforeEach() {
    fakeClock.setTimeTo(Clock.System.now())
    notRejectedBefore = fakeClock.now().minus(1.hours)
    rejectedTransactionsPostgresDao =
      RejectedTransactionsPostgresDao(
        readConnection = sqlClient,
        writeConnection = sqlClient,
        clock = fakeClock
      )
  }

  private fun performInsertTest(
    rejectedTransaction: RejectedTransaction
  ) {
    rejectedTransactionsPostgresDao.saveNewRejectedTransaction(rejectedTransaction).get()

    // assert the corresponding record was inserted into the full_transactions table
    val newlyInsertedFullTxnsRows = dbTableContentQuery(RejectedTransactionsPostgresDao.fullTransactionsTable)
      .execute().get().filter { row ->
        row.getBuffer("tx_hash").bytes.contentEquals(rejectedTransaction.transactionInfo.hash)
      }
    assertThat(newlyInsertedFullTxnsRows.size).isEqualTo(1)
    assertThat(newlyInsertedFullTxnsRows.first().getBuffer("tx_rlp").bytes).isEqualTo(
      rejectedTransaction.transactionRLP
    )

    // assert the corresponding record was inserted into the rejected_transactions table
    val newlyInsertedRejectedTxnsRows = dbTableContentQuery(RejectedTransactionsPostgresDao.rejectedTransactionsTable)
      .execute().get().filter { row ->
        row.getBuffer("tx_hash").bytes.contentEquals(rejectedTransaction.transactionInfo.hash) &&
          row.getString("reject_reason") == rejectedTransaction.reasonMessage
      }
    assertThat(newlyInsertedRejectedTxnsRows.size).isEqualTo(1)
    val insertedRow = newlyInsertedRejectedTxnsRows.first()
    assertThat(insertedRow.getLong("created_epoch_milli")).isEqualTo(
      fakeClock.now().toEpochMilliseconds()
    )
    assertThat(insertedRow.getString("reject_stage")).isEqualTo(
      RejectedTransactionsPostgresDao.rejectedStageToDbValue(rejectedTransaction.txRejectionStage)
    )
    assertThat(insertedRow.getLong("block_number")?.toULong()).isEqualTo(
      rejectedTransaction.blockNumber
    )
    assertThat(insertedRow.getJsonArray("overflows").encode()).isEqualTo(
      ObjectMapper().writeValueAsString(rejectedTransaction.overflows)
    )
    assertThat(insertedRow.getLong("reject_timestamp")).isEqualTo(
      rejectedTransaction.timestamp.toEpochMilliseconds()
    )
    assertThat(insertedRow.getBuffer("tx_from").bytes).isEqualTo(
      rejectedTransaction.transactionInfo.from
    )
    assertThat(insertedRow.getBuffer("tx_to")?.bytes).isEqualTo(
      rejectedTransaction.transactionInfo.to
    )
    assertThat(insertedRow.getLong("tx_nonce")).isEqualTo(
      rejectedTransaction.transactionInfo.nonce.toLong()
    )
  }

  @Test
  fun `saveNewRejectedTransaction inserts new rejected transaction to db`() {
    // insert a new rejected transaction
    performInsertTest(createRejectedTransaction())

    // assert that the total number of rows in the two tables are correct
    assertThat(rejectedTransactionsTotalRows()).isEqualTo(1)
    assertThat(fullTransactionsTotalRows()).isEqualTo(1)
  }

  @Test
  fun `saveNewRejectedTransaction inserts new rejected contract deployment transaction to db`() {
    // insert a new rejected contract deployment transaction (with "transactionInfo.to" as null)
    performInsertTest(rejectedContractDeploymentTransaction)

    // assert that the total number of rows in the two tables are correct
    assertThat(rejectedTransactionsTotalRows()).isEqualTo(1)
    assertThat(fullTransactionsTotalRows()).isEqualTo(1)
  }

  @Test
  fun `saveNewRejectedTransaction inserts new rejected transactions with same txHash but different reason to db`() {
    // insert a new rejected transaction
    performInsertTest(createRejectedTransaction())

    // insert another rejected transaction with same txHash but different reason
    performInsertTest(
      createRejectedTransaction(
        txRejectionStage = RejectedTransaction.Stage.P2P,
        blockNumber = null,
        reasonMessage = "Transaction line count for module MUL=587 is above the limit 401",
        overflows = listOf(
          ModuleOverflow(
            module = "MUL",
            count = 587,
            limit = 401
          )
        )
      )
    )

    // assert that the total number of rows in the two tables are correct
    assertThat(rejectedTransactionsTotalRows()).isEqualTo(2)
    assertThat(fullTransactionsTotalRows()).isEqualTo(1)
  }

  @Test
  fun `saveNewRejectedTransaction throws error when inserting rejected transactions with same txHash and reason`() {
    // insert a new rejected transaction
    performInsertTest(createRejectedTransaction())

    // another rejected transaction with same txHash and reason
    val duplicatedRejectedTransaction = createRejectedTransaction(
      txRejectionStage = RejectedTransaction.Stage.P2P,
      blockNumber = null,
      overflows = listOf(
        ModuleOverflow(
          module = "ADD",
          count = 587,
          limit = 401
        )
      )
    )

    // assert that the insertion of duplicatedRejectedTransaction would trigger DuplicatedRecordException error
    assertThrows<ExecutionException> {
      rejectedTransactionsPostgresDao.saveNewRejectedTransaction(duplicatedRejectedTransaction).get()
    }.also { executionException ->
      assertThat(executionException.cause).isInstanceOf(DuplicatedRecordException::class.java)
      assertThat(executionException.cause!!.message)
        .isEqualTo(
          "RejectedTransaction ${duplicatedRejectedTransaction.transactionInfo.hash.encodeHex()} " +
            "is already persisted!"
        )
    }

    // assert that the total number of rows in the two tables are correct
    assertThat(rejectedTransactionsTotalRows()).isEqualTo(1)
    assertThat(fullTransactionsTotalRows()).isEqualTo(1)
  }

  @Test
  fun `findRejectedTransactionByTxHash returns rejected transaction with most recent timestamp from db`() {
    // insert a new rejected transaction
    val oldestRejectedTransaction = createRejectedTransaction(
      timestamp = fakeClock.now().minus(10.seconds)
    )
    performInsertTest(oldestRejectedTransaction)

    // insert another rejected transaction with same txHash but different reason and
    // with a more recent timestamp
    performInsertTest(
      createRejectedTransaction(
        reasonMessage = "Transaction line count for module MUL=587 is above the limit 401",
        timestamp = fakeClock.now().minus(9.seconds)
      )
    )

    // insert another rejected transaction with same txHash but different reason
    // and with the most recent timestamp
    val newestRejectedTransaction = createRejectedTransaction(
      reasonMessage = "Transaction line count for module EXP=9000 is above the limit 8192",
      timestamp = fakeClock.now().minus(8.seconds)
    )
    performInsertTest(newestRejectedTransaction)

    // find the rejected transaction with the txHash
    val foundRejectedTransaction = rejectedTransactionsPostgresDao.findRejectedTransactionByTxHash(
      oldestRejectedTransaction.transactionInfo.hash
    ).get()

    // assert that the found rejected transaction is the same as the one with most recent timestamp
    assertThat(foundRejectedTransaction).isEqualTo(newestRejectedTransaction)

    // assert that the total number of rows in the two tables are correct
    assertThat(rejectedTransactionsTotalRows()).isEqualTo(3)
    assertThat(fullTransactionsTotalRows()).isEqualTo(1)
  }

  @Test
  fun `findRejectedTransactionByTxHash returns null as rejected timestamp exceeds queryable window`() {
    // insert a new rejected transaction with timestamp exceeds the 1-hour queryable window
    val rejectedTransaction = createRejectedTransaction(
      timestamp = fakeClock.now().minus(1.hours).minus(1.seconds)
    )
    performInsertTest(rejectedTransaction)

    // find the rejected transaction with the txHash
    val foundRejectedTransaction = rejectedTransactionsPostgresDao.findRejectedTransactionByTxHash(
      rejectedTransaction.transactionInfo.hash,
      notRejectedBefore
    ).get()

    // assert that null is returned from the find method
    assertThat(foundRejectedTransaction).isNull()

    // assert that the total number of rows in the two tables are both one which
    // implies the rejected transaction is still present in db
    assertThat(rejectedTransactionsTotalRows()).isEqualTo(1)
    assertThat(fullTransactionsTotalRows()).isEqualTo(1)
  }

  @Test
  fun `deleteRejectedTransactions returns 2 row deleted as created timestamp exceeds storage window`() {
    // insert a new rejected transaction A
    performInsertTest(
      createRejectedTransaction(
        transactionInfo = TransactionInfo(
          hash = "0x078ecd6f00bff4beca9116ca85c65ddd265971e415d7df7a96b3c10424b031e2".decodeHex(),
          from = "0x4d144d7b9c96b26361d6ac74dd1d8267edca4fc2".decodeHex(),
          to = "0x1195cf65f83b3a5768f3c496d3a05ad6412c64b3".decodeHex(),
          nonce = 101UL
        ),
        reasonMessage = "Transaction line count for module EXP=10000 is above the limit 8192",
        timestamp = fakeClock.now()
      )
    )
    // advance the fake clock to make its created timestamp exceeds the 10-hours storage window
    fakeClock.advanceBy(1.hours)

    // insert another rejected transaction B1
    performInsertTest(
      createRejectedTransaction(
        timestamp = fakeClock.now()
      )
    )
    // advance the fake clock to make its created timestamp exceeds the 10-hours storage window
    fakeClock.advanceBy(1.hours)

    // insert another rejected transaction B2 with same txHash as B1 but different reason
    performInsertTest(
      createRejectedTransaction(
        reasonMessage = "Transaction line count for module EXP=9000 is above the limit 8192",
        timestamp = fakeClock.now()
      )
    )
    // advance the fake clock to make its created timestamp stay within the 10-hours storage window
    fakeClock.advanceBy(10.hours)

    // assert that the total number of rows in the two tables are 3 and 2 respectively
    // which implies all the rejected transactions above are present in db
    assertThat(rejectedTransactionsTotalRows()).isEqualTo(3)
    assertThat(fullTransactionsTotalRows()).isEqualTo(2)

    // delete the rejected transactions with storage window as 10 hours from now
    val deletedRows = rejectedTransactionsPostgresDao.deleteRejectedTransactions(
      fakeClock.now().minus(10.hours)
    ).get()

    // assert that number of total deleted rows in rejected_transactions table is 2
    assertThat(deletedRows).isEqualTo(2)

    // assert that the total number of rows in the two tables are both 1 which
    // implies only the rejected transaction A and B1 and A's corresponding full transaction
    // were deleted due to created timestamp exceeds the storage window
    assertThat(rejectedTransactionsTotalRows()).isEqualTo(1)
    assertThat(fullTransactionsTotalRows()).isEqualTo(1)
  }
}
