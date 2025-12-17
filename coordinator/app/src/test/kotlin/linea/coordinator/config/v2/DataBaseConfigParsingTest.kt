package linea.coordinator.config.v2

import com.sksamuel.hoplite.Masked
import linea.coordinator.config.v2.toml.DatabaseToml
import linea.coordinator.config.v2.toml.RequestRetriesToml
import linea.coordinator.config.v2.toml.parseConfig
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import kotlin.time.Duration.Companion.minutes
import kotlin.time.Duration.Companion.seconds

class DataBaseConfigParsingTest {
  companion object {
    val toml =
      """
      [database]
      hostname = "localhost"
      port = "5432"
      username = "someuser"
      password = "somepassword"
      schema = "linea_coordinator"
      read_pool_size = 10
      read_pipelining_limit = 11
      transactional_pool_size = 12
      [database.persistence-retries]
      max-retries = 3
      backoff-delay = "PT1S"
      timeout = "PT40S"
      failures-warning-threshold = 2
      """.trimIndent()

    val config =
      DatabaseToml(
        hostname = "localhost",
        username = "someuser",
        password = Masked("somepassword"),
        schema = "linea_coordinator",
        readPoolSize = 10,
        readPipeliningLimit = 11,
        transactionalPoolSize = 12,
        port = 5432u,
        persistenceRetries =
        RequestRetriesToml(
          maxRetries = 3u,
          backoffDelay = 1.seconds,
          timeout = 40.seconds,
          failuresWarningThreshold = 2u,
        ),
      )

    val tomlMinimal =
      """
      [database]
      hostname = "localhost"
      username = "someuser"
      password = "somepassword"
      """.trimIndent()

    val configMinimal =
      DatabaseToml(
        hostname = "localhost",
        username = "someuser",
        password = Masked("somepassword"),
        schema = "linea_coordinator",
        readPoolSize = 10,
        readPipeliningLimit = 10,
        transactionalPoolSize = 10,
        port = 5432u,
        persistenceRetries =
        RequestRetriesToml(
          maxRetries = null,
          backoffDelay = 1.seconds,
          timeout = 10.minutes,
          failuresWarningThreshold = 3u,
        ),
      )
  }

  data class WrapperConfig(
    val database: DatabaseToml,
  )

  @Test
  fun `should parse database full config`() {
    assertThat(parseConfig<WrapperConfig>(toml).database).isEqualTo(config)
  }

  @Test
  fun `should parse database minimal config`() {
    assertThat(parseConfig<WrapperConfig>(tomlMinimal).database).isEqualTo(configMinimal)
  }
}
