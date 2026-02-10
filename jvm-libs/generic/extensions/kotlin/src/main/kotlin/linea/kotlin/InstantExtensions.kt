package linea.kotlin

import kotlin.time.Instant

fun Instant.trimToMillisecondPrecision(): Instant {
  return Instant.fromEpochMilliseconds(this.toEpochMilliseconds())
}

fun Instant.trimToSecondPrecision(): Instant {
  return Instant.fromEpochSeconds(this.epochSeconds)
}

fun Instant.trimToMinutePrecision(): Instant {
  return Instant.fromEpochSeconds(this.epochSeconds - this.epochSeconds % 60)
}
