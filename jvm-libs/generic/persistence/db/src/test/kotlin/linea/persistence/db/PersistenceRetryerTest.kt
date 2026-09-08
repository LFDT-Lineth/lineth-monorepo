package linea.persistence.db

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
import java.time.Clock
import java.time.Instant
import java.time.ZoneId
import java.time.ZoneOffset
import java.util.concurrent.ExecutionException
import java.util.concurrent.atomic.AtomicInteger
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds
import kotlin.time.toJavaDuration
import java.time.Duration as JavaDuration

private class MutableClock(
  private var currentInstant: Instant,
) : Clock() {
  override fun getZone(): ZoneId = ZoneOffset.UTC

  override fun withZone(zone: ZoneId?): Clock = this

  override fun instant(): Instant = currentInstant

  fun advanceBy(duration: JavaDuration) {
    currentInstant = currentInstant.plus(duration)
  }
}

@ExtendWith(VertxExtension::class)
class PersistenceRetryerTest {
  private lateinit var persistenceRetryer: PersistenceRetryer
  private lateinit var listAppender: ListAppender
  private lateinit var clock: MutableClock

  @BeforeEach
  fun setup(vertx: Vertx) {
    val ctx = LogManager.getContext(false) as LoggerContext
    listAppender = ctx.configuration.getAppender("ListAppender") as ListAppender
    listAppender.clear()

    clock = MutableClock(Instant.parse("2026-09-08T00:00:00Z"))
    persistenceRetryer = PersistenceRetryer(
      vertx = vertx,
      config = PersistenceRetryer.Config(
        backoffDelay = 10.milliseconds,
        maxRetries = 10,
        timeout = 2.seconds,
        ignoreFirstExceptionsUntilTimeElapsed = 80.milliseconds,
      ),
      clock = clock,
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

    assertThrows<ExecutionException> {
      persistenceRetryer.retryQuery<Unit>(
        action = {
          when (callCounter.incrementAndGet()) {
            1 -> SafeFuture.failedFuture<Unit>(pgErrorBeforeMute)
            else -> {
              clock.advanceBy(JavaDuration.ofMillis(100))
              SafeFuture.failedFuture<Unit>(pgErrorAfterMute)
            }
          }
        },
      ).get()
    }

    val loggedMessages = listAppender.events.map { it.message.formattedMessage }
    assertThat(loggedMessages).noneMatch { it.contains(pgErrorBeforeMute.message!!) }
    assertThat(loggedMessages).anyMatch { it.contains(pgErrorAfterMute.message!!) }
  }
}
