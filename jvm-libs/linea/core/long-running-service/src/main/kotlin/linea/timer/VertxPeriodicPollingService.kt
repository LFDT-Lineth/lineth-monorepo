package linea.timer

import io.vertx.core.Vertx
import org.apache.logging.log4j.Logger
import kotlin.time.Duration.Companion.milliseconds

abstract class VertxPeriodicPollingService(
  private val vertx: Vertx,
  private val pollingIntervalMs: Long,
  private val log: Logger,
  private val name: String,
  timerSchedule: TimerSchedule,
) : PeriodicPollingService(
  timerFactory = VertxTimerFactory(vertx),
  timerSchedule = timerSchedule,
  pollingInterval = pollingIntervalMs.milliseconds,
  log = log,
  name = name,
) {
  override fun handleError(error: Throwable) {
    log.error("Error with period polling service={}: errorMessage={}", name, error.message, error)
  }
}
