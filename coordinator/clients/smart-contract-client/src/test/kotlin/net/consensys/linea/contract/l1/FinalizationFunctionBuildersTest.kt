package net.consensys.linea.contract.l1

import linea.contract.l1.LineaValidiumContractVersion
import linea.contract.l1.LinethRollupContractVersion
import linea.domain.createBlobRecord
import linea.domain.createProofToFinalize
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Test

class FinalizationFunctionBuildersTest {
  private val blob = createBlobRecord(startBlockNumber = 1UL, endBlockNumber = 2UL)
  private val aggregation = createProofToFinalize(firstBlockNumber = 1L, finalBlockNumber = 2L)

  @Test
  fun `builds V6 finalization with a compression proof`() {
    assertThat(
      Web3JLinethRollupFunctionBuilders.buildFinalizeBlocksFunction(
        LinethRollupContractVersion.V6,
        aggregation,
        blob,
        ByteArray(32),
        0L,
      ),
    ).isNotNull()
  }

  @Test
  fun `builds V8 finalization with a compression proof`() {
    assertThat(
      FunctionBuildersV8.buildFinalizeBlocksFunctionV8(
        aggregation,
        blob,
        ByteArray(32),
        0L,
      ),
    ).isNotNull()
  }

  @Test
  fun `builds Validium finalization with a compression proof`() {
    assertThat(
      Web3JLineaValidiumFunctionBuilders.buildFinalizeBlocksFunction(
        LineaValidiumContractVersion.V1,
        aggregation,
        blob,
        ByteArray(32),
        0L,
      ),
    ).isNotNull()
  }

  @Test
  fun `rejects finalization without a compression proof`() {
    val unprovenBlob = blob.copy(blobCompressionProof = null)

    listOf<() -> Unit>(
      {
        Web3JLinethRollupFunctionBuilders.buildFinalizeBlocksFunction(
          LinethRollupContractVersion.V6,
          aggregation,
          unprovenBlob,
          ByteArray(32),
          0L,
        )
      },
      {
        FunctionBuildersV8.buildFinalizeBlocksFunctionV8(
          aggregation,
          unprovenBlob,
          ByteArray(32),
          0L,
        )
      },
      {
        Web3JLineaValidiumFunctionBuilders.buildFinalizeBlocksFunction(
          LineaValidiumContractVersion.V1,
          aggregation,
          unprovenBlob,
          ByteArray(32),
          0L,
        )
      },
    ).forEach { buildFunction ->
      assertThatThrownBy(buildFunction)
        .isInstanceOf(IllegalArgumentException::class.java)
        .hasMessage(
          "aggregationLastBlob.blobCompressionProof must be set when building the finalization function",
        )
    }
  }
}
