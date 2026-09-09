package lineth.contract.l1

import linea.contract.ValidiumV1
import linea.contract.ValidiumV2
import linea.contract.l1.LineaValidiumContractVersion
import linea.contract.l1.LinethRollupContractVersion
import linea.domain.createBlobRecord
import linea.domain.createProofToFinalize
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.assertThrows
import org.web3j.abi.FunctionEncoder
import org.web3j.tx.TransactionManager

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
    assertThat(
      Web3JLineaValidiumFunctionBuilders.buildFinalizeBlocksFunction(
        LineaValidiumContractVersion.V2,
        aggregation,
        blob,
        ByteArray(32),
        0L,
      ),
    ).isNotNull()
  }

  @Test
  fun `Validium V2 finalization parameters encode identically to rollup V8`() {
    // Validium V2's FinalizationDataV4 is the same tuple as rollup V8's, so the (positionally
    // encoded) parameter payload must match the battle-tested V8 builder byte for byte. This
    // pins the struct field ORDER, the one thing that can silently corrupt the calldata.
    // Every V4-specific field gets a DISTINCT non-zero value so a swapped or substituted argument
    // (e.g. the two ftx numbers, or the parent vs final ftx rolling hash) changes the encoding.
    val distinctlyPopulatedAggregation = createProofToFinalize(
      firstBlockNumber = 1L,
      finalBlockNumber = 2L,
      parentAggregationFtxNumber = 7UL,
      finalFtxNumber = 9UL,
      parentAggregationFtxRollingHash = ByteArray(32) { 0x0a },
      finalFtxRollingHash = ByteArray(32) { 0x0b },
      filteredAddresses = listOf(ByteArray(20) { 0x0c }, ByteArray(20) { 0x0d }),
    ).copy(
      // createProofToFinalize leaves these zero; make them distinct too so a swap of the adjacent
      // uint256(0) members (l1RollingHashMessageNumber vs l2MerkleTreesDepth) can't pass silently.
      aggregatedVerifierIndex = 1,
      l1RollingHash = ByteArray(32) { 0x0f },
      l1RollingHashMessageNumber = 4L,
      l2MerkleTreesDepth = 5,
    )
    val parentL1RollingHash = ByteArray(32) { 0x0e }
    val parentL1RollingHashMessageNumber = 3L

    val v2 = FunctionEncoder.encode(
      Web3JLineaValidiumFunctionBuilders.buildFinalizeBlockFunctionV2(
        distinctlyPopulatedAggregation,
        blob,
        parentL1RollingHash,
        parentL1RollingHashMessageNumber,
      ),
    )
    val v8 = FunctionEncoder.encode(
      FunctionBuildersV8.buildFinalizeBlocksFunctionV8(
        distinctlyPopulatedAggregation,
        blob,
        parentL1RollingHash,
        parentL1RollingHashMessageNumber,
      ),
    )
    // strip the 4-byte selectors ("0x" + 8 hex chars); the parameter encoding must be identical
    assertThat(v2.substring(10)).isEqualTo(v8.substring(10))
  }

  @Test
  fun `Validium acceptShnarfData is unchanged between V1 and V2`() {
    // We reuse the V1 encoding for V2 because acceptShnarfData is identical in both ABIs. The wrapper
    // FUNC_ constants only carry the function NAME, so comparing them would pass even if the argument
    // lists diverged; compare full encoded calldata instead, using the independently-generated V1 and
    // V2 wrappers as the reference encoders (encodeFunctionCall builds calldata without a connection).
    val prevShnarf = ByteArray(32) { 0x0a }
    val expectedShnarf = ByteArray(32) { 0x0b }
    val finalStateRootHash = ByteArray(32) { 0x0c }

    val zeroAddress = "0x0000000000000000000000000000000000000000"
    val v1Reference = ValidiumV1.load(zeroAddress, null, null as TransactionManager?, null)
      .acceptShnarfData(prevShnarf, expectedShnarf, finalStateRootHash)
      .encodeFunctionCall()
    val v2Reference = ValidiumV2.load(zeroAddress, null, null as TransactionManager?, null)
      .acceptShnarfData(prevShnarf, expectedShnarf, finalStateRootHash)
      .encodeFunctionCall()
    // The two generated wrappers must agree (name, argument types and order), and our shared builder
    // must produce those exact bytes for both versions.
    assertThat(v2Reference).isEqualTo(v1Reference)

    val blobWithDistinctShnarfData = blob.copy(
      blobCompressionProof = blob.blobCompressionProof!!.copy(
        prevShnarf = prevShnarf,
        expectedShnarf = expectedShnarf,
        finalStateRootHash = finalStateRootHash,
      ),
    )
    listOf(LineaValidiumContractVersion.V1, LineaValidiumContractVersion.V2).forEach { version ->
      val encoded = FunctionEncoder.encode(
        Web3JLineaValidiumFunctionBuilders.buildAcceptShnarfDataFunction(version, listOf(blobWithDistinctShnarfData)),
      )
      assertThat(encoded).isEqualTo(v1Reference)
    }
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
      {
        Web3JLineaValidiumFunctionBuilders.buildFinalizeBlocksFunction(
          LineaValidiumContractVersion.V2,
          aggregation,
          unprovenBlob,
          ByteArray(32),
          0L,
        )
      },
    ).forEach { buildFunction ->
      val exception = assertThrows<IllegalArgumentException> { buildFunction() }
      assertThat(exception)
        .hasMessage(
          "aggregationLastBlob.blobCompressionProof must be set when building the finalization function",
        )
    }
  }
}
