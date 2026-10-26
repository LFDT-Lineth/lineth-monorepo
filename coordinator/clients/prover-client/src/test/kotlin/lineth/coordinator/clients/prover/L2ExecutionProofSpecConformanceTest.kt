package lineth.coordinator.clients.prover

import com.fasterxml.jackson.databind.JsonNode
import com.fasterxml.jackson.databind.node.ObjectNode
import lineth.coordinator.clients.prover.RollupSpecProverIo.assertSchemaValid
import lineth.coordinator.clients.prover.RollupSpecProverIo.fixture
import lineth.coordinator.clients.prover.RollupSpecProverIo.fixtureAs
import lineth.coordinator.clients.prover.RollupSpecProverIo.schemaErrors
import lineth.coordinator.clients.prover.RollupSpecProverIo.toJson
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import tech.pegasys.teku.infrastructure.async.SafeFuture

/**
 * Checks the l2-execution DTOs against the rollup_spec wire contract (see [RollupSpecProverIo]). Strict fixture
 * round-trips are covered for all proof types by [RollupSpecFixturesConformanceTest].
 */
class L2ExecutionProofSpecConformanceTest {
  private val requestSchema = "getZkL2ExecutionProofV1.request.schema.json"

  @Test
  fun `l2-execution response fixture maps to the domain`() {
    val json = fixture("10-11-getZkL2ExecutionProofV1.response.json")

    val dto = fixtureAs<L2ExecutionProofResponseDto>("10-11-getZkL2ExecutionProofV1.response.json")

    assertThat(dto.proverVersion).isEqualTo(json["proverVersion"].asText())
    val domain = L2ExecutionProofResponseDtoMapper(dto)
    assertThat(domain.endBlockNumber).isEqualTo(json["publicInputs"]["endBlockNumber"].asLong().toULong())
    assertThat(domain.publicInputs.blockCount).isEqualTo(json["publicInputs"]["blockCount"].asLong().toULong())
  }

  @Test
  fun `l2-execution request built by the mapper is valid against the request schema`() {
    val dto = L2ExecutionProofRequestDtoMapper(
      guestProgramId = RiscvProverClientTestFixtures.L2_EXECUTION_PROGRAM_ID,
      provingSystem = RiscvProverClientTestFixtures.PROVING_SYSTEM_VERSION,
      l2MessageServiceAddress = RiscvProverClientTestFixtures.L2_MESSAGE_SERVICE_ADDRESS,
      forkName = RiscvProverClientTestFixtures.FORK_NAME,
    ).invoke(RiscvProverClientTestFixtures.l2ExecutionProofRequestV1()).get()

    val json = toJson(dto)
    assertSchemaValid(requestSchema, json)

    // Guard against a vacuous pass: the pre-spec envelope key must be rejected.
    val legacyEnvelope = (json.deepCopy<JsonNode>() as ObjectNode).apply {
      set<JsonNode>("programId", remove("guestProgramId"))
    }
    assertThat(schemaErrors(requestSchema, legacyEnvelope)).isNotEmpty()
  }

  @Test
  fun `l2-execution proof embedded in a rollup request carries no proverVersion`() {
    val response = RiscvProverClientTestFixtures.l2ExecutionProofResponseDto(1000501L, 1000520L)
    assertThat(response.proverVersion).isNotNull()

    val dto = RollupProofRequestDtoMapper(
      guestProgramId = RiscvProverClientTestFixtures.ROLLUP_PROGRAM_ID,
      provingSystem = RiscvProverClientTestFixtures.PROVING_SYSTEM_VERSION,
      chainId = RiscvProverClientTestFixtures.CHAIN_ID,
      l2ExecutionProofProvider = { _ -> SafeFuture.completedFuture(response) },
    ).invoke(RiscvProverClientTestFixtures.rollupProofRequestV1()).get()

    val embedded = toJson(dto)["proofRequest"]["l2ExecutionProofs"]
    assertThat(embedded).hasSize(1)
    assertThat(embedded[0].has("proverVersion")).isFalse()
  }
}
