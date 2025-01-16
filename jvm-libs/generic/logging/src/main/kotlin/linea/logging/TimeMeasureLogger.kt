package linea.logging

import org.apache.logging.log4j.Level
import org.apache.logging.log4j.Logger
import kotlin.time.measureTime

/**
 * This is more for debugging purposes, to measure the time taken by a certain action and log it.
 * For production code, consider using metrics.
 */
fun <T> measureTimeAndLog(
  logger: Logger,
  logLevel: Level = Level.DEBUG,
  logMessageProvider: (duration: kotlin.time.Duration, result: T) -> String,
  action: () -> T
): T {
  var value: T
  val duration = measureTime {
    value = action()
  }
  if (logger.isEnabled(logLevel)) {
    logger.log(logLevel, logMessageProvider(duration, value))
  }
  return value
}

class MeasureLogger(
  private val logger: Logger,
  private val logLevel: Level = Level.DEBUG
) {
  fun <T> measureTimeAndLog(
    logLevel: Level = this.logLevel,
    logMessageProvider: (duration: kotlin.time.Duration, result: T) -> String,
    action: () -> T
  ): T {
    return measureTimeAndLog(logger, logLevel, logMessageProvider, action)
  }
}
