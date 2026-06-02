package linea.executionwitness

import com.github.michaelbull.result.getOrElse
import linea.domain.BlockParameter
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test

class FakeExecutionWitnessClientTest {

  @Test
  fun `getExecutionWitness should find witness by block hash key`() {
    val blockHash = BlockParameter.fromHash(ByteArray(32) { 3 })
    val witness = ExecutionWitness(
      state = listOf(byteArrayOf(1)),
      keys = emptyList(),
      codes = emptyList(),
      headers = emptyList(),
    )
    val client = FakeExecutionWitnessClient(
      witnessesByBlock = mapOf(blockHash to witness),
    )

    val result = client.getExecutionWitness(BlockParameter.fromHash(blockHash.getHash().copyOf())).get()
      .getOrElse { error("unexpected error: $it") }

    assertThat(result).isEqualTo(witness)
  }
}
