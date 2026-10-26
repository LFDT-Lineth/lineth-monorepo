package lineth.coordinator.clients.prover

import com.fasterxml.jackson.databind.DeserializationFeature
import com.fasterxml.jackson.databind.JsonNode
import com.fasterxml.jackson.databind.ObjectMapper
import com.networknt.schema.JsonSchema
import com.networknt.schema.JsonSchemaFactory
import com.networknt.schema.SpecVersion
import lineth.coordinator.clients.prover.serialization.JsonSerialization
import org.assertj.core.api.Assertions.assertThat
import java.nio.file.Files
import java.nio.file.Path

/**
 * Access to the rollup_spec wire contract for conformance tests: the golden fixtures under
 * `rollup_spec/src/rollup_spec/prover_io/testdata/` and the Draft 2020-12 schemas under `.../prover_io/schemas/`.
 *
 * [strictMapper] enables `FAIL_ON_UNKNOWN_PROPERTIES` (the production mapper disables it), so a field added to the
 * spec but missing from a DTO fails the test instead of being silently dropped.
 */
object RollupSpecProverIo {
  private val proverIoDir: Path = Path.of(
    requireNotNull(System.getProperty("rollupSpecProverIoDir")) {
      "system property rollupSpecProverIoDir must point to rollup_spec/src/rollup_spec/prover_io"
    },
  )
  val mapper: ObjectMapper = JsonSerialization.proofResponseMapperV1
  val strictMapper: ObjectMapper = mapper.copy()
    .configure(DeserializationFeature.FAIL_ON_UNKNOWN_PROPERTIES, true)

  fun fixture(name: String): JsonNode = mapper.readTree(proverIoDir.resolve("testdata/$name").toFile())

  /** Every golden fixture file name under `testdata/`, sorted. */
  fun fixtureNames(): List<String> = Files.list(proverIoDir.resolve("testdata")).use { files ->
    files.map { it.fileName.toString() }.filter { it.endsWith(".json") }.sorted().toList()
  }

  inline fun <reified T> fixtureAs(name: String): T = strictMapper.treeToValue(fixture(name), T::class.java)

  fun schema(name: String): JsonSchema = Files.newInputStream(proverIoDir.resolve("schemas/$name")).use {
    JsonSchemaFactory.getInstance(SpecVersion.VersionFlag.V202012).getSchema(it)
  }

  /** Re-parses the serialized DTO so numbers compare by value (`IntNode` vs `LongNode`) against a parsed fixture. */
  fun toJson(dto: Any): JsonNode = mapper.readTree(mapper.writeValueAsString(dto))

  fun schemaErrors(schemaName: String, json: JsonNode): List<String> =
    schema(schemaName).validate(json).map { it.toString() }

  fun assertSchemaValid(schemaName: String, json: JsonNode) {
    assertThat(schemaErrors(schemaName, json)).isEmpty()
  }
}
