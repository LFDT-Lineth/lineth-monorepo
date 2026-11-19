package linea.kotlin

import kotlin.time.Instant

/** This value, a block timestamp or any other count of seconds since the Unix epoch, as an [Instant]. */
fun ULong.toInstantFromEpochSeconds(): Instant = Instant.fromEpochSeconds(this.toLong())

fun Instant.trimToMillisecondPrecision(): Instant {
  return Instant.fromEpochMilliseconds(this.toEpochMilliseconds())
}

fun Instant.trimToSecondPrecision(): Instant {
  return Instant.fromEpochSeconds(this.epochSeconds)
}

fun Instant.trimToMinutePrecision(): Instant {
  return Instant.fromEpochSeconds(this.epochSeconds - this.epochSeconds % 60)
}
