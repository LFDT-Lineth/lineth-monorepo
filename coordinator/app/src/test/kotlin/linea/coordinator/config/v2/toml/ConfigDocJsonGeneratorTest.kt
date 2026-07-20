package linea.coordinator.config.v2.toml

import com.fasterxml.jackson.databind.JsonNode
import com.fasterxml.jackson.databind.ObjectMapper
import linea.coordinator.config.v2.docs.ConfigDocJsonGenerator
import linea.coordinator.config.v2.docs.CoordinatorConfigFileRoot
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test

class ConfigDocJsonGeneratorTest {
  data class SampleFileToml(
    @param:ConfigDoc("The hostname.", example = "localhost")
    val hostname: String,
    @param:ConfigDoc("The port.", default = "5432")
    val port: UInt = 5432u,
    @param:ConfigSection("Nested settings.")
    val nested: NestedToml = NestedToml(),
  ) {
    data class NestedToml(
      @param:ConfigDoc("Max attempts.", default = "3")
      val maxAttempts: UInt = 3u,
    )
  }

  private val files = listOf(
    CoordinatorConfigFileRoot("sample", "A sample file.", SampleFileToml::class),
  )
  private val mapper = ObjectMapper()

  @Test
  fun `emits valid JSON organised per file then per key`() {
    val tree = mapper.readTree(ConfigDocJsonGenerator.generate(files))

    val keys = tree.path("sample").path("keys")
    assertThat(tree.path("sample").path("description").asText()).isEqualTo("A sample file.")
    assertThat(keys.fieldNames().asSequence().toList()).containsExactly(
      "hostname",
      "nested",
      "nested.max-attempts",
      "port",
    )
  }

  @Test
  fun `serialises leaf fields including declared default and null for unset`() {
    val port = mapper.readTree(ConfigDocJsonGenerator.generate(files)).at("/sample/keys/port")

    assertThat(port.path("type").asText()).isEqualTo("UInt")
    assertThat(port.path("section").asBoolean()).isFalse()
    assertThat(port.path("required").asBoolean()).isFalse()
    assertThat(port.path("default").asText()).isEqualTo("5432")
    assertThat(port.path("example").isNull).isTrue()
    assertThat(port.path("replacement").isNull).isTrue()
  }

  @Test
  fun `marks sections with the section flag`() {
    val nested = mapper.readTree(ConfigDocJsonGenerator.generate(files)).at("/sample/keys/nested")
    assertThat(nested.path("section").asBoolean()).isTrue()
    assertThat(nested.path("type").asText()).isEqualTo("NestedToml")
  }

  @Test
  fun `real config produces one entry per file with sorted keys and a trailing newline`() {
    val output = ConfigDocJsonGenerator.generate()
    val tree: JsonNode = mapper.readTree(output)

    assertThat(output).endsWith("\n")
    assertThat(tree.fieldNames().asSequence().toList()).containsExactly(
      "coordinator",
      "traces-limits-v4",
      "traces-limits-v5",
      "gas-price-cap-time-of-day-multipliers",
      "smart-contract-errors",
    )
    val coordinatorKeys = tree.at("/coordinator/keys").fieldNames().asSequence().toList()
    assertThat(coordinatorKeys).isSorted
  }
}
