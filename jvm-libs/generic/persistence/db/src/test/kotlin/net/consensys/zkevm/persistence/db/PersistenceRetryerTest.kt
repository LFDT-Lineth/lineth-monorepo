package net.consensys.zkevm.persistence.db

import io.vertx.core.Vertx
import io.vertx.junit5.VertxExtension
import io.vertx.pgclient.PgException
import org.apache.logging.log4j.LogManager
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.assertThrows
import org.junit.jupiter.api.extension.ExtendWith
import org.mockito.Mockito
import org.mockito.kotlin.atLeastOnce
import org.mockito.kotlin.eq
import org.mockito.kotlin.never
import org.mockito.kotlin.verify
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.time.Instant
import java.util.concurrent.ExecutionException
import java.util.concurrent.atomic.AtomicInteger
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds
import kotlin.time.toJavaDuration

@ExtendWith(VertxExtension::class)
class PersistenceRetryerTest {
  private lateinit var persistenceRetryer: PersistenceRetryer
  private val mockedLog = Mockito.spy(LogManager.getLogger(PersistenceRetryer::class.java))

  @BeforeEach
  fun setup(vertx: Vertx) {
    persistenceRetryer = PersistenceRetryer(
      vertx = vertx,
      config = PersistenceRetryer.Config(
        backoffDelay = 10.milliseconds,
        maxRetries = 5,
        timeout = 2.seconds,
        ignoreFirstExceptionsUntilTimeElapsed = 80.milliseconds,
      ),
      log = mockedLog,
    )
  }

  @Test
  fun persistenceRetryerRetriesUntilSuccessful() {
    val callCounter = AtomicInteger(0)
    assertThat(
      persistenceRetryer.retryQuery(
        action = {
          if (callCounter.incrementAndGet() < 2) {
            throw RuntimeException()
          } else {
            SafeFuture.completedFuture("success")
          }
        },
      ),
    ).succeedsWithin(1.seconds.toJavaDuration())
      .isEqualTo("success")
    assertThat(callCounter.get()).isEqualTo(2)
  }

  @Test
  fun persistenceRetryerStopRetriesOnDuplicateKeyError() {
    val callCounter = AtomicInteger(0)

    assertThat(
      persistenceRetryer.retryQuery(
        action = {
          if (callCounter.incrementAndGet() < 20) {
            SafeFuture.failedFuture(
              PgException(
                "duplicate key value violates unique constraint",
                "some-severity",
                "some-code",
                "some-detail",
              ),
            )
          } else {
            SafeFuture.completedFuture("success")
          }
        },
      ),
    ).isCompletedExceptionally
      .isNotCancelled
      .failsWithin(2.seconds.toJavaDuration())
      .withThrowableOfType(ExecutionException::class.java)
      .withCauseInstanceOf(PgException::class.java)

    assertThat(callCounter.get()).isEqualTo(1)
  }

  @Test
  fun persistenceRetryerMuteErrorLogsFor80ms() {
    val callCounter = AtomicInteger(0)
    val pgErrorBeforeMute = PgException(
      "Unknown error before mute",
      "some-severity",
      "some-code",
      "some-detail",
    )
    val pgErrorAfterMute = PgException(
      "Unknown error after mute",
      "some-severity",
      "some-code",
      "some-detail",
    )
    val startTime = Instant.now()

    assertThrows<ExecutionException> {
      persistenceRetryer.retryQuery(
        action = {
          if (callCounter.incrementAndGet() < 20) {
            if ((Instant.now().toEpochMilli() - startTime.toEpochMilli()) <= 80.milliseconds.inWholeMilliseconds) {
              SafeFuture.failedFuture(pgErrorBeforeMute)
            } else {
              SafeFuture.failedFuture(pgErrorAfterMute)
            }
          } else {
            SafeFuture.completedFuture("success")
          }
        },
      ).get()
    }

    assertThat(callCounter.get()).isEqualTo(6)
    verify(mockedLog, never()).warn(
      eq("Persistence errorMessage={}, it will retry again in {}"),
      eq(pgErrorBeforeMute.message),
      eq(10.milliseconds),
      eq(pgErrorBeforeMute),
    )
    verify(mockedLog, atLeastOnce()).warn(
      eq("Persistence errorMessage={}, it will retry again in {}"),
      eq(pgErrorAfterMute.message),
      eq(10.milliseconds),
      eq(pgErrorAfterMute),
    )
  }
}
