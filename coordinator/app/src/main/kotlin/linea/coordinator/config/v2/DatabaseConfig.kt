package linea.coordinator.config.v2

import com.sksamuel.hoplite.Masked
import linea.domain.RetryConfig
import kotlin.time.Duration.Companion.minutes
import kotlin.time.Duration.Companion.seconds

data class DatabaseConfig(
  val host: String,
  val port: Int,
  val username: String,
  val password: Masked,
  val schema: String,
  val schemaVersion: Int = 4,
  val readPoolSize: Int = 10,
  val readPipeliningLimit: Int = 10,
  val transactionalPoolSize: Int = 10,
  val persistenceRetries: RetryConfig =
    RetryConfig(
      backoffDelay = 1.seconds,
      timeout = 10.minutes,
      failuresWarningThreshold = 3u,
    ),
) {
  companion object {
    val supportedSchemas = 4..5
  }

  init {
    require(schemaVersion in supportedSchemas) {
      "schemaVersion=$schemaVersion must be between ${supportedSchemas.first} and ${supportedSchemas.last}"
    }
  }
}
