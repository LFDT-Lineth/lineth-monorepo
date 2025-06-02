package net.consensys.zkevm.persistence.db

import io.vertx.core.Vertx
import io.vertx.junit5.VertxExtension
import io.vertx.pgclient.PgException
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.ExecutionException
import java.util.concurrent.atomic.AtomicInteger
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds
import kotlin.time.toJavaDuration

@ExtendWith(VertxExtension::class)
class PersistenceRetryerTest {
  private lateinit var persistenceRetryer: PersistenceRetryer

  @BeforeEach
  fun setup(vertx: Vertx) {
    persistenceRetryer = PersistenceRetryer(
      vertx = vertx,
      PersistenceRetryer.Config(
        backoffDelay = 10.milliseconds,
        maxRetries = 5,
        timeout = 2.seconds,
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
  fun persistenceRetryerStropRetriesOnDuplicateKeyError() {
    val callCounter = AtomicInteger(0)

    assertThat(
      persistenceRetryer.retryQuery(
        action = {
          if (callCounter.incrementAndGet() < 20) {
            SafeFuture.failedFuture<String>(
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
}
