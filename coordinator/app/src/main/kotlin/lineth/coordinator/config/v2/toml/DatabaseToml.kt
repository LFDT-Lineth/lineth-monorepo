package lineth.coordinator.config.v2.toml

import com.sksamuel.hoplite.Masked
import lineth.coordinator.config.v2.DatabaseConfig
import lineth.coordinator.config.v2.DatabaseConfig.Companion.supportedSchemas
import lineth.config.docs.ConfigDoc
import lineth.config.docs.ConfigSection
import kotlin.time.Duration.Companion.minutes
import kotlin.time.Duration.Companion.seconds

data class DatabaseToml(
  @param:ConfigDoc(
    description = "PostgreSQL hostname used by the coordinator persistence layer.",
    example = "postgres",
  )
  val hostname: String,
  @param:ConfigDoc(
    description = "PostgreSQL port.",
    default = "5432",
  )
  val port: UInt = 5432u,
  @param:ConfigDoc(
    description = "PostgreSQL username.",
    example = "postgres",
  )
  val username: String,
  @param:ConfigDoc(description = "PostgreSQL password. Masked in logs.")
  val password: Masked,
  @param:ConfigDoc(
    description = "PostgreSQL schema (database) name.",
    default = "linea_coordinator",
  )
  val schema: String = "linea_coordinator",
  @param:ConfigDoc(
    description = "Expected database schema version; must match a supported migration version.",
    default = "4",
  )
  val schemaVersion: Int = 4,
  @param:ConfigDoc(
    description = "Connection pool size for read-only queries.",
    default = "10",
  )
  val readPoolSize: Int = 10,
  @param:ConfigDoc(
    description = "Maximum number of read queries pipelined on a single connection.",
    default = "10",
  )
  val readPipeliningLimit: Int = 10,
  @param:ConfigDoc(
    description = "Connection pool size for transactional (read-write) queries.",
    default = "10",
  )
  val transactionalPoolSize: Int = 10,
  @param:ConfigSection("Retry policy for database persistence operations.")
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
