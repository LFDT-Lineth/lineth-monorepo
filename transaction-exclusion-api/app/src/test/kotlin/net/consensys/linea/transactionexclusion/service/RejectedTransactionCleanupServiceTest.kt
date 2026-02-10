package net.consensys.linea.transactionexclusion.service

import io.vertx.core.Vertx
import io.vertx.junit5.Timeout
import io.vertx.junit5.VertxExtension
import io.vertx.junit5.VertxTestContext
import net.consensys.FakeFixedClock
import net.consensys.zkevm.persistence.dao.rejectedtransaction.RejectedTransactionsDao
import org.awaitility.Awaitility
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import org.mockito.Mockito
import org.mockito.kotlin.any
import org.mockito.kotlin.atLeastOnce
import org.mockito.kotlin.eq
import org.mockito.kotlin.mock
import org.mockito.kotlin.verify
import org.mockito.kotlin.whenever
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.TimeUnit
import kotlin.time.Clock
import kotlin.time.Duration.Companion.hours
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.toJavaDuration

@ExtendWith(VertxExtension::class)
class RejectedTransactionCleanupServiceTest {
  private lateinit var rejectedTransactionCleanupService: RejectedTransactionCleanupService
  private lateinit var rejectedTransactionsRepositoryMock: RejectedTransactionsDao
  private var fakeClock = FakeFixedClock(Clock.System.now())

  @BeforeEach
  fun beforeEach() {
    fakeClock.setTimeTo(Clock.System.now())
    rejectedTransactionsRepositoryMock = mock<RejectedTransactionsDao>(
      defaultAnswer = Mockito.RETURNS_DEEP_STUBS,
    ).also {
      whenever(it.deleteRejectedTransactions(any()))
        .thenReturn(SafeFuture.completedFuture(1))
    }
    rejectedTransactionCleanupService =
      RejectedTransactionCleanupService(
        config = RejectedTransactionCleanupService.Config(
          pollingInterval = 100.milliseconds,
          storagePeriod = 24.hours,
        ),
        clock = fakeClock,
        vertx = Vertx.vertx(),
        repository = rejectedTransactionsRepositoryMock,
      )
  }

  @Test
  @Timeout(2, timeUnit = TimeUnit.SECONDS)
  fun `when rejectedTransactionCleanupService starts, deleteRejectedTransaction should be called`(
    testContext: VertxTestContext,
  ) {
    rejectedTransactionCleanupService.start()
      .thenApply {
        Awaitility.await()
          .pollInterval(50.milliseconds.toJavaDuration())
          .untilAsserted {
            verify(rejectedTransactionsRepositoryMock, atLeastOnce())
              .deleteRejectedTransactions(eq(fakeClock.now().minus(24.hours)))
          }
        testContext.completeNow()
      }.whenException(testContext::failNow)
  }
}
