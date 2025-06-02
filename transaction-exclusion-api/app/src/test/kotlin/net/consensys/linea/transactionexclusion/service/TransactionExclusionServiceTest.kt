package net.consensys.linea.transactionexclusion.service

import com.github.michaelbull.result.Err
import com.github.michaelbull.result.Ok
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.linea.transactionexclusion.ErrorType
import net.consensys.linea.transactionexclusion.TransactionExclusionError
import net.consensys.linea.transactionexclusion.TransactionExclusionServiceV1
import net.consensys.linea.transactionexclusion.test.defaultRejectedTransaction
import net.consensys.zkevm.persistence.dao.rejectedtransaction.RejectedTransactionsDao
import net.consensys.zkevm.persistence.db.DuplicatedRecordException
import org.junit.jupiter.api.Assertions
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.mockito.Mockito
import org.mockito.kotlin.any
import org.mockito.kotlin.mock
import org.mockito.kotlin.whenever
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Duration.Companion.hours

class TransactionExclusionServiceTest {
  private val metricsFacadeMock = mock<MetricsFacade>(defaultAnswer = Mockito.RETURNS_DEEP_STUBS)
  private val config = TransactionExclusionServiceV1Impl.Config(
    rejectedTimestampWithinDuration = 24.hours,
  )
  private lateinit var rejectedTransactionsRepositoryMock: RejectedTransactionsDao

  @BeforeEach
  fun beforeEach() {
    rejectedTransactionsRepositoryMock = mock<RejectedTransactionsDao>(
      defaultAnswer = Mockito.RETURNS_DEEP_STUBS,
    )
  }

  @Test
  fun saveRejectedTransaction_return_success_result_with_saved_status() {
    whenever(rejectedTransactionsRepositoryMock.findRejectedTransactionByTxHash(any(), any()))
      .thenReturn(SafeFuture.completedFuture(defaultRejectedTransaction))
    whenever(rejectedTransactionsRepositoryMock.saveNewRejectedTransaction(any()))
      .thenReturn(SafeFuture.completedFuture(Unit))

    val transactionExclusionService = TransactionExclusionServiceV1Impl(
      config = config,
      repository = rejectedTransactionsRepositoryMock,
      metricsFacade = metricsFacadeMock,
    )

    Assertions.assertEquals(
      Ok(TransactionExclusionServiceV1.SaveRejectedTransactionStatus.SAVED),
      transactionExclusionService.saveRejectedTransaction(defaultRejectedTransaction).get(),
    )
  }

  @Test
  fun saveRejectedTransaction_return_success_result_with_duplicated_already_saved_status() {
    whenever(rejectedTransactionsRepositoryMock.saveNewRejectedTransaction(any()))
      .thenReturn(SafeFuture.failedFuture(DuplicatedRecordException()))

    val transactionExclusionService = TransactionExclusionServiceV1Impl(
      config = config,
      repository = rejectedTransactionsRepositoryMock,
      metricsFacade = metricsFacadeMock,
    )

    Assertions.assertEquals(
      Ok(TransactionExclusionServiceV1.SaveRejectedTransactionStatus.DUPLICATE_ALREADY_SAVED_BEFORE),
      transactionExclusionService.saveRejectedTransaction(defaultRejectedTransaction).get(),
    )
  }

  @Test
  fun saveRejectedTransaction_return_error_result_when_saveRejectedTransaction_failed() {
    whenever(rejectedTransactionsRepositoryMock.findRejectedTransactionByTxHash(any(), any()))
      .thenReturn(SafeFuture.completedFuture(defaultRejectedTransaction))
    whenever(rejectedTransactionsRepositoryMock.saveNewRejectedTransaction(any()))
      .thenReturn(SafeFuture.failedFuture(RuntimeException()))

    val transactionExclusionService = TransactionExclusionServiceV1Impl(
      config = config,
      repository = rejectedTransactionsRepositoryMock,
      metricsFacade = metricsFacadeMock,
    )

    Assertions.assertEquals(
      Err(TransactionExclusionError(ErrorType.SERVER_ERROR, "")),
      transactionExclusionService.saveRejectedTransaction(defaultRejectedTransaction).get(),
    )
  }

  @Test
  fun getTransactionExclusionStatus_return_success_result_with_rejected_txn() {
    whenever(rejectedTransactionsRepositoryMock.findRejectedTransactionByTxHash(any(), any()))
      .thenReturn(SafeFuture.completedFuture(defaultRejectedTransaction))

    val transactionExclusionService = TransactionExclusionServiceV1Impl(
      config = config,
      repository = rejectedTransactionsRepositoryMock,
      metricsFacade = metricsFacadeMock,
    )

    Assertions.assertEquals(
      Ok(defaultRejectedTransaction),
      transactionExclusionService.getTransactionExclusionStatus(
        defaultRejectedTransaction.transactionInfo.hash,
      ).get(),
    )
  }

  @Test
  fun getTransactionExclusionStatus_return_error_result_with_transaction_unavailable() {
    whenever(rejectedTransactionsRepositoryMock.findRejectedTransactionByTxHash(any(), any()))
      .thenReturn(SafeFuture.completedFuture(null))

    val transactionExclusionService = TransactionExclusionServiceV1Impl(
      config = config,
      repository = rejectedTransactionsRepositoryMock,
      metricsFacade = metricsFacadeMock,
    )

    Assertions.assertEquals(
      Ok(null),
      transactionExclusionService.getTransactionExclusionStatus(
        defaultRejectedTransaction.transactionInfo.hash,
      ).get(),
    )
  }

  @Test
  fun getTransactionExclusionStatus_return_error_result_with_other_error() {
    whenever(rejectedTransactionsRepositoryMock.findRejectedTransactionByTxHash(any(), any()))
      .thenReturn(SafeFuture.failedFuture(RuntimeException()))

    val transactionExclusionService = TransactionExclusionServiceV1Impl(
      config = config,
      repository = rejectedTransactionsRepositoryMock,
      metricsFacade = metricsFacadeMock,
    )

    Assertions.assertEquals(
      Err(
        TransactionExclusionError(
          ErrorType.SERVER_ERROR,
          "",
        ),
      ),
      transactionExclusionService.getTransactionExclusionStatus(
        defaultRejectedTransaction.transactionInfo.hash,
      ).get(),
    )
  }
}
