package lineth.coordinator.clients.prover

import lineth.coordinator.clients.prover.RollupSpecProverIo.assertSchemaValid
import lineth.coordinator.clients.prover.RollupSpecProverIo.fixture
import lineth.coordinator.clients.prover.RollupSpecProverIo.fixtureAs
import lineth.coordinator.clients.prover.RollupSpecProverIo.schemaErrors
import lineth.coordinator.clients.prover.RollupSpecProverIo.toJson
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Test
import tech.pegasys.teku.infrastructure.async.SafeFuture

/**
 * Checks the rollup-aggregation DTOs against the rollup_spec wire contract (see [RollupSpecProverIo]). Strict fixture
 * round-trips are covered for all proof types by [RollupSpecFixturesConformanceTest].
 */
class RollupAggregationProofSpecConformanceTest {
  private val requestSchema = "getZkRollupAggregationProofV1.request.schema.json"
  private val rollupResponse = fixtureAs<RollupProofResponseDto>("10-14-getZkRollupProofV1.response.json")
  private val aggregationRequest = RiscvProverClientTestFixtures.rollupAggregationProofRequestV1()

  private fun aggregationRequestMapper(withProvider: Boolean) = RollupAggregationProofRequestDtoMapper(
    guestProgramId = RiscvProverClientTestFixtures.ROLLUP_AGGREGATION_PROGRAM_ID,
    provingSystem = RiscvProverClientTestFixtures.PROVING_SYSTEM_VERSION,
    rollupProofProvider = if (withProvider) {
      { _ -> SafeFuture.completedFuture(rollupResponse) }
    } else {
      null
    },
  )

  @Test
  fun `aggregation response fixture maps to the finalization public inputs`() {
    val pi = fixture("10-18-getZkRollupAggregationProofV1.response.json")["publicInputs"]

    val domain = RollupAggregationProofResponseDtoMapper(
      fixtureAs<RollupAggregationProofResponseDto>("10-18-getZkRollupAggregationProofV1.response.json"),
    )

    assertThat(domain.endBlockNumber).isEqualTo(pi["endBlockNumber"].asLong().toULong())
    assertThat(domain.publicInputs.l2L1TreeDepth).isEqualTo(pi["l2L1TreeDepth"].asLong().toULong())
    assertThat(domain.publicInputs.l2L1Roots).hasSize(pi["l2L1Roots"].size())
    assertThat(domain.publicInputs.programIds).hasSize(pi["programIds"].size())
    assertThat(domain.publicInputs.l2MessagingBlocksOffsets).hasSize(pi["l2MessagingBlocksOffsets"].size())
  }

  @Test
  fun `spec-mode aggregation request built by the mapper is valid against the request schema`() {
    val json = toJson(aggregationRequestMapper(withProvider = true).invoke(aggregationRequest).get())

    assertSchemaValid(requestSchema, json)
    val proofRequest = json["proofRequest"]
    assertThat(proofRequest.has("rollupProofsIndexes")).isFalse()
    assertThat(proofRequest["rollupProofs"][0].has("proverVersion")).isFalse()
  }

  @Test
  fun `gateway-mode aggregation request sends only indexes and is not a spec request`() {
    val json = toJson(aggregationRequestMapper(withProvider = false).invoke(aggregationRequest).get())

    val proofRequest = json["proofRequest"]
    assertThat(proofRequest.has("rollupProofs")).isFalse()
    assertThat(proofRequest["rollupProofsIndexes"]).hasSize(aggregationRequest.rollupProofs.size)
    assertThat(schemaErrors(requestSchema, json)).isNotEmpty()
  }

  @Test
  fun `aggregation request params require exactly one of inlined proofs or indexes`() {
    assertThatThrownBy { FileBasedRollupAggregationProofRequestParamsDto() }
      .isInstanceOf(IllegalArgumentException::class.java)
    assertThatThrownBy {
      FileBasedRollupAggregationProofRequestParamsDto(
        rollupProofs = listOf(rollupResponse),
        rollupProofsIndexes = listOf(BlockIntervalDto(1, 2)),
      )
    }.isInstanceOf(IllegalArgumentException::class.java)
  }
}
