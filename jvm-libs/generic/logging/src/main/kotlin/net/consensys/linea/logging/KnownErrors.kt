package net.consensys.linea.logging

import org.apache.logging.log4j.Level

data class KnownError(
  val logLevel: Level,
  val message: Regex,
  val stackTrace: Boolean = false,
)

class KnownErrors(
  private val knownErrors: List<KnownError>,
) {
  fun find(errorMessage: String): KnownError? {
    return knownErrors.firstOrNull { errorMessage.matches(it.message) }
  }
}
