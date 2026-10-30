package lineth.coordinator.clients.prover

import linea.clients.ConflationWitness
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
 * Checks the rollup DTOs against the rollup_spec wire contract (see [RollupSpecProverIo]). Strict fixture round-trips
 * are covered for all proof types by [RollupSpecFixturesConformanceTest].
 */
class RollupProofSpecConformanceTest {
  private val requestSchema = "getZkRollupProofV1.request.schema.json"
  private val l2ExecutionResponse =
    fixtureAs<L2ExecutionProofResponseDto>("10-11-getZkL2ExecutionProofV1.response.json")

  private fun rollupRequestMapper(withProvider: Boolean) = RollupProofRequestDtoMapper(
    guestProgramId = RiscvProverClientTestFixtures.ROLLUP_PROGRAM_ID,
    provingSystem = RiscvProverClientTestFixtures.PROVING_SYSTEM_VERSION,
    chainId = RiscvProverClientTestFixtures.CHAIN_ID,
    l2ExecutionProofProvider = if (withProvider) {
      { _ -> SafeFuture.completedFuture(l2ExecutionResponse) }
    } else {
      null
    },
  )

  private val rollupRequest = RiscvProverClientTestFixtures.rollupProofRequestV1(
    conflations = listOf(ConflationWitness(blockRlps = listOf(byteArrayOf(0x3c)))),
  )

  @Test
  fun `rollup response fixture maps to the domain`() {
    val json = fixture("10-14-getZkRollupProofV1.response.json")

    val dto = fixtureAs<RollupProofResponseDto>("10-14-getZkRollupProofV1.response.json")

    val domain = RollupProofResponseDtoMapper(dto)
    val pi = json["publicInputs"]
    assertThat(domain.endBlockNumber).isEqualTo(pi["endBlockNumber"].asLong().toULong())
    assertThat(domain.publicInputs.parentDataTailTakeBytes).isEqualTo(pi["parentDataTailTakeBytes"].asInt())
    assertThat(domain.publicInputs.finalDataTailDiscardBytes).isEqualTo(pi["finalDataTailDiscardBytes"].asInt())
    assertThat(domain.publicInputs.l2L1Messages).hasSize(pi["l2L1Messages"].size())
    assertThat(domain.publicInputs.programVks).hasSize(pi["programVks"].size())
  }

  @Test
  fun `rollup request fixture covers a mid-chunk start`() {
    val dto = fixtureAs<FileBasedRollupProofRequestDto>("10-14-getZkRollupProofV1.request.json")

    assertThat(dto.proofRequest.parentDataTailTakeBytes).isPositive()
    assertThat(dto.proofRequest.boundaryPrevDataRollingHash).isNotNull()
  }

  @Test
  fun `spec-mode rollup request built by the mapper is valid against the request schema`() {
    val json = toJson(rollupRequestMapper(withProvider = true).invoke(rollupRequest).get())

    assertSchemaValid(requestSchema, json)
    val proofRequest = json["proofRequest"]
    assertThat(proofRequest.has("l2ExecutionProofIndexes")).isFalse()
    // parentDataTailTakeBytes = 0, so the optional boundary hash is omitted rather than written as null.
    assertThat(proofRequest.has("boundaryPrevDataRollingHash")).isFalse()
    assertThat(proofRequest["l2ExecutionProofs"][0].has("proverVersion")).isFalse()
  }

  @Test
  fun `blob and calldata chunks built by the mapper are valid against the request schema`() {
    val request = RiscvProverClientTestFixtures.rollupProofRequestV1(
      conflations = listOf(ConflationWitness(blockRlps = listOf(byteArrayOf(0x3c)))),
      chunks = listOf(
        RiscvProverClientTestFixtures.blobChunkWitness(),
        RiscvProverClientTestFixtures.calldataChunkWitness(),
      ),
    )

    val json = toJson(rollupRequestMapper(withProvider = true).invoke(request).get())

    assertSchemaValid(requestSchema, json)
    val (blobChunk, calldataChunk) = json["proofRequest"]["chunks"].toList()
    assertThat(blobChunk["kzgCommitment"].asText()).isEqualTo("0x" + "21".repeat(48))
    assertThat(blobChunk["kzgProof"].asText()).isEqualTo("0x" + "22".repeat(48))
    assertThat(calldataChunk["kzgCommitment"].asText()).isEqualTo("0x")
    assertThat(calldataChunk["kzgProof"].asText()).isEqualTo("0x")
  }

  @Test
  fun `gateway-mode rollup request sends only indexes and is not a spec request`() {
    val json = toJson(rollupRequestMapper(withProvider = false).invoke(rollupRequest).get())

    val proofRequest = json["proofRequest"]
    assertThat(proofRequest.has("l2ExecutionProofs")).isFalse()
    assertThat(proofRequest["l2ExecutionProofIndexes"]).hasSize(rollupRequest.l2Executions.size)
    assertThat(schemaErrors(requestSchema, json)).isNotEmpty()
  }

  @Test
  fun `rollup request params require exactly one of inlined proofs or indexes`() {
    val chunk = RiscvProverClientTestFixtures.blobChunkWitness().toDto()
    fun params(proofs: List<L2ExecutionProofResponseDto>?, indexes: List<BlockIntervalDto>?) =
      RollupProofRequestParamsDto(
        chainId = RiscvProverClientTestFixtures.CHAIN_ID,
        conflations = emptyList(),
        l2ExecutionProofs = proofs,
        l2ExecutionProofIndexes = indexes,
        chunks = listOf(chunk),
        parentDataRollingHash = "0x" + "19".repeat(32),
        parentDataTailTakeBytes = 0,
      )

    assertThatThrownBy { params(proofs = null, indexes = null) }
      .isInstanceOf(IllegalArgumentException::class.java)
    assertThatThrownBy { params(proofs = listOf(l2ExecutionResponse), indexes = listOf(BlockIntervalDto(1, 2))) }
      .isInstanceOf(IllegalArgumentException::class.java)
  }

  @Test
  fun `rollup proof embedded in an aggregation request carries no proverVersion`() {
    val response = RiscvProverClientTestFixtures.rollupProofResponseDto(1000501L, 1000520L)
    assertThat(response.proverVersion).isNotNull()

    val dto = RollupAggregationProofRequestDtoMapper(
      guestProgramId = RiscvProverClientTestFixtures.ROLLUP_AGGREGATION_PROGRAM_ID,
      provingSystem = RiscvProverClientTestFixtures.PROVING_SYSTEM_VERSION,
      rollupProofProvider = { _ -> SafeFuture.completedFuture(response) },
    ).invoke(RiscvProverClientTestFixtures.rollupAggregationProofRequestV1()).get()

    val embedded = toJson(dto)["proofRequest"]["rollupProofs"]
    assertThat(embedded).hasSize(1)
    assertThat(embedded[0].has("proverVersion")).isFalse()
  }
}
