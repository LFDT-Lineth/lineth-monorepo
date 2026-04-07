package net.consensys.zkevm.persistence.db

import io.vertx.core.Vertx
import io.vertx.junit5.VertxExtension
import io.vertx.pgclient.PgException
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.core.LoggerContext
import org.apache.logging.log4j.core.test.appender.ListAppender
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.assertThrows
import org.junit.jupiter.api.extension.ExtendWith
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.ExecutionException
import java.util.concurrent.atomic.AtomicInteger
import kotlin.time.Clock
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds
import kotlin.time.toJavaDuration

@ExtendWith(VertxExtension::class)
class PersistenceRetryerTest {
  private lateinit var persistenceRetryer: PersistenceRetryer
  private lateinit var listAppender: ListAppender

  @BeforeEach
  fun setup(vertx: Vertx) {
    val ctx = LogManager.getContext(false) as LoggerContext
    listAppender = ctx.configuration.getAppender("ListAppender") as ListAppender
    listAppender.clear()

    persistenceRetryer = PersistenceRetryer(
      vertx = vertx,
      config = PersistenceRetryer.Config(
        backoffDelay = 10.milliseconds,
        maxRetries = 10,
        timeout = 2.seconds,
        ignoreFirstExceptionsUntilTimeElapsed = 80.milliseconds,
      ),
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
    val startTime = Clock.System.now()

    assertThrows<ExecutionException> {
      persistenceRetryer.retryQuery(
        action = {
          if (callCounter.incrementAndGet() < 20) {
            if (Clock.System.now().minus(startTime) < 80.milliseconds) {
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

    val loggedMessages = listAppender.events.map { it.message.formattedMessage }
    assertThat(loggedMessages).noneMatch { it.contains(pgErrorBeforeMute.message!!) }
    assertThat(loggedMessages).anyMatch { it.contains(pgErrorAfterMute.message!!) }
  }
}
