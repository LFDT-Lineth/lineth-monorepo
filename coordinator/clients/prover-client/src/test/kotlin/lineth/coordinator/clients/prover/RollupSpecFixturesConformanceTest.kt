package lineth.coordinator.clients.prover

import lineth.coordinator.clients.prover.RollupSpecProverIo.fixture
import lineth.coordinator.clients.prover.RollupSpecProverIo.fixtureNames
import lineth.coordinator.clients.prover.RollupSpecProverIo.strictMapper
import lineth.coordinator.clients.prover.RollupSpecProverIo.toJson
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import org.junit.jupiter.params.ParameterizedTest
import org.junit.jupiter.params.provider.MethodSource

/**
 * Regression guard over every golden fixture in `rollup_spec/src/rollup_spec/prover_io/testdata/`: each one must
 * deserialize strictly into its coordinator DTO and serialize back to the identical JSON. Fixtures are discovered
 * from disk, so a fixture added to the spec without a matching DTO fails here.
 */
class RollupSpecFixturesConformanceTest {
  companion object {
    /** Fixture name (minus the `<start>-<end>-` block-range prefix) -> the DTO it deserializes into. */
    private val dtoByFixture: Map<String, Class<*>> = mapOf(
      "getZkL2ExecutionProofV1.request.json" to L2ExecutionProofRequestDto::class.java,
      "getZkL2ExecutionProofV1.response.json" to L2ExecutionProofResponseDto::class.java,
      "getZkRollupProofV1.request.json" to FileBasedRollupProofRequestDto::class.java,
      "getZkRollupProofV1.response.json" to RollupProofResponseDto::class.java,
      "getZkRollupAggregationProofV1.request.json" to FileBasedRollupAggregationProofRequestDto::class.java,
      "getZkRollupAggregationProofV1.response.json" to RollupAggregationProofResponseDto::class.java,
    )

    private fun fixtureKind(name: String): String = name.replace(Regex("^\\d+-\\d+-"), "")

    @JvmStatic
    fun fixtures(): List<String> = fixtureNames()
  }

  @Test
  fun `every V1 request and response has a golden fixture`() {
    assertThat(fixtureNames().map(::fixtureKind)).containsExactlyInAnyOrderElementsOf(dtoByFixture.keys)
  }

  @ParameterizedTest(name = "{0}")
  @MethodSource("fixtures")
  fun `golden fixture round-trips through its DTO`(fixtureName: String) {
    val dtoClass = requireNotNull(dtoByFixture[fixtureKind(fixtureName)]) {
      "no DTO mapped for fixture $fixtureName; add it to dtoByFixture"
    }
    val json = fixture(fixtureName)

    val dto = strictMapper.treeToValue(json, dtoClass)

    assertThat(toJson(dto)).isEqualTo(json)
  }
}
