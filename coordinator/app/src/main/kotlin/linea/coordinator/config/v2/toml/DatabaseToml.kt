package linea.coordinator.config.v2.toml

import com.sksamuel.hoplite.Masked
import linea.coordinator.config.v2.DatabaseConfig
import linea.coordinator.config.v2.DatabaseConfig.Companion.supportedSchemas
import kotlin.time.Duration.Companion.minutes
import kotlin.time.Duration.Companion.seconds

data class DatabaseToml(
  val hostname: String,
  val port: UInt = 5432u,
  val username: String,
  val password: Masked,
  val schema: String = "linea_coordinator",
  val schemaVersion: Int = 4,
  val readPoolSize: Int = 10,
  val readPipeliningLimit: Int = 10,
  val transactionalPoolSize: Int = 10,
  val persistenceRetries: RequestRetriesToml =
    RequestRetriesToml(
      backoffDelay = 1.seconds,
      timeout = 10.minutes,
      failuresWarningThreshold = 3u,
    ),
) {
  init {
    require(schemaVersion in supportedSchemas) {
      "schemaVersion=$schemaVersion must be between ${supportedSchemas.first} and ${supportedSchemas.last}"
    }
  }
  fun reified(): DatabaseConfig {
    return DatabaseConfig(
      host = this.hostname,
      port = this.port.toInt(),
      username = this.username,
      password = this.password,
      schema = this.schema,
      schemaVersion = this.schemaVersion,
      readPoolSize = this.readPoolSize,
      readPipeliningLimit = this.readPipeliningLimit,
      transactionalPoolSize = this.transactionalPoolSize,
      persistenceRetries = this.persistenceRetries.asDomain,
    )
  }
}
