package linea.timer

import linea.LongRunningService
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Duration
import kotlin.time.Duration.Companion.milliseconds

abstract class PeriodicPollingService(
  private val timerFactory: TimerFactory,
  private val timerSchedule: TimerSchedule,
  private val pollingInterval: Duration,
  private val initialDelay: Duration = if (timerFactory is VertxTimerFactory) 1.milliseconds else Duration.ZERO,
  private val log: Logger,
  private val name: String,
) : LongRunningService {
  init {
    require(pollingInterval.inWholeMilliseconds > 0) {
      "pollingInterval must be greater than 0"
    }
  }
  private var timer: Timer? = null

  abstract fun action(): SafeFuture<*>

  open fun handleError(error: Throwable) {
    log.warn("Error with periodic polling service={}: errorMessage={}", name, error.message, error)
  }

  @Synchronized
  override fun start(): SafeFuture<Unit> {
    if (timer == null) {
      timer = timerFactory.createTimer(
        name = name,
        task = { action().get() },
        initialDelay = initialDelay,
        period = pollingInterval,
        timerSchedule = timerSchedule,
        errorHandler = this::handleError,
      )
      timer!!.start()
    }
    return SafeFuture.completedFuture(Unit)
  }

  @Synchronized
  override fun stop(): SafeFuture<Unit> {
    if (timer != null) {
      timer!!.stop()
      timer = null
    } else {
      log.info("Service is not running to stop it!")
    }
    return SafeFuture.completedFuture(Unit)
  }
}
