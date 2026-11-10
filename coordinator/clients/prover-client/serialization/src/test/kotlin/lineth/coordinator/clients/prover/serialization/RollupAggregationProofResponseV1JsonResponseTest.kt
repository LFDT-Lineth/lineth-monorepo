package lineth.coordinator.clients.prover.serialization

import linea.clients.createRollupAggregationProofResponseV1
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test

class RollupAggregationProofResponseV1JsonResponseTest {
  private val response = createRollupAggregationProofResponseV1(
    startBlockNumber = 101UL,
    endBlockNumber = 160UL,
    l2L1Roots = listOf(ByteArray(32) { 0x0a }, ByteArray(32) { 0x0b }),
    filteredAddresses = listOf(ByteArray(20) { 0x0c }),
    l2MessagingBlocksOffsets = listOf(3UL, 17UL),
  )

  @Test
  fun `domain object round-trips through the JSON response`() {
    val jsonResponse = RollupAggregationProofResponseV1JsonResponse.fromDomainObject(response)

    assertThat(jsonResponse.toDomainObject()).isEqualTo(response)
  }

  @Test
  fun `byte fields are written as 0x-prefixed hex`() {
    val json = JsonSerialization.proofResponseMapperV1.readTree(
      RollupAggregationProofResponseV1JsonResponse.fromDomainObject(response).toJsonString(),
    )

    assertThat(json["endBlockHash"].asText()).startsWith("0x").hasSize(66)
    assertThat(json["l2L1Roots"][0].asText()).isEqualTo("0x" + "0a".repeat(32))
  }
}
