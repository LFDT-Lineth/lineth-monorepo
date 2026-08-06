package linea.coordinator.clients.prover.riscv

import linea.domain.BlockIntervalProofIndex
import linea.kotlin.encodeHex
import tech.pegasys.teku.infrastructure.async.SafeFuture

/**
 * Fake [L2ExecutionProofTransport] that always reports the request as submitted and returns a fixed
 * [L2ExecutionProofResponseDto]. Used to drive `FileBasedRollupProverClient`'s request mapper without a real
 * l2-execution proof transport.
 */
class FakeL2ExecutionProofTransport(
  private val responseProvider: ((BlockIntervalProofIndex) -> L2ExecutionProofResponseDto?)? = null,
) : L2ExecutionProofTransport {
  private val l2ExecutionProofResponseDto = L2ExecutionProofResponseDto(
    startBlockNumber = 1L,
    proverVersion = RiscVProverClientTestFixtures.PROVER_VERSION,
    proof = byteArrayOf(0x02).encodeHex(),
    publicInputs = RiscVProverClientTestFixtures.l2ExecutionProofPublicInputsDto(2L),
    l2L1Messages = listOf(ByteArray(32) { 0x4a }.encodeHex()),
    txFroms = listOf(ByteArray(20) { 0x4b }.encodeHex()),
    filteredAddresses = listOf(ByteArray(20) { 0x4c }.encodeHex()),
  )
  override fun isRequestAlreadySubmitted(proofIndex: BlockIntervalProofIndex): SafeFuture<Boolean> =
    SafeFuture.completedFuture(true)

  override fun submitRequest(
    proofIndex: BlockIntervalProofIndex,
    requestDto: L2ExecutionProofRequestDto,
  ): SafeFuture<Unit> = SafeFuture.completedFuture(Unit)

  override fun findResponse(proofIndex: BlockIntervalProofIndex): SafeFuture<L2ExecutionProofResponseDto?> =
    SafeFuture.completedFuture(response(proofIndex))

  override fun awaitResponse(proofIndex: BlockIntervalProofIndex): SafeFuture<L2ExecutionProofResponseDto> =
    SafeFuture.completedFuture(requireNotNull(response(proofIndex)))

  private fun response(proofIndex: BlockIntervalProofIndex): L2ExecutionProofResponseDto? =
    if (responseProvider != null) responseProvider(proofIndex) else l2ExecutionProofResponseDto(proofIndex)

  private fun l2ExecutionProofResponseDto(
    proofIndex: BlockIntervalProofIndex,
  ): L2ExecutionProofResponseDto {
    return l2ExecutionProofResponseDto.copy(
      startBlockNumber = proofIndex.startBlockNumber.toLong(),
      publicInputs = RiscVProverClientTestFixtures.l2ExecutionProofPublicInputsDto(
        proofIndex.endBlockNumber.toLong(),
      ),
    )
  }
}

/**
 * Fake [FileBasedRollupProofTransport] that always reports the request as submitted and returns a fixed
 * [RollupProofResponseDto]. Used to drive `FileBasedRollupAggregationProverClient`'s request mapper without a real
 * rollup proof transport.
 */
class FakeRollupProofTransport(
  private val responseProvider: ((BlockIntervalProofIndex) -> RollupProofResponseDto?)? = null,
) : FileBasedRollupProofTransport {
  private val rollupProofResponseDto = RollupProofResponseDto(
    startBlockNumber = 1L,
    proverVersion = RiscVProverClientTestFixtures.PROVER_VERSION,
    proof = byteArrayOf(0x4a).encodeHex(),
    publicInputs = RiscVProverClientTestFixtures.rollupProofPublicInputsDto(2L),
    l2L1Roots = listOf(ByteArray(32) { 0x5a }.encodeHex()),
    filteredAddresses = listOf(ByteArray(20) { 0x5b }.encodeHex()),
  )
  override fun isRequestAlreadySubmitted(proofIndex: BlockIntervalProofIndex): SafeFuture<Boolean> =
    SafeFuture.completedFuture(true)

  override fun submitRequest(
    proofIndex: BlockIntervalProofIndex,
    requestDto: FileBasedRollupProofRequestDto,
  ): SafeFuture<Unit> = SafeFuture.completedFuture(Unit)

  override fun findResponse(proofIndex: BlockIntervalProofIndex): SafeFuture<RollupProofResponseDto?> =
    SafeFuture.completedFuture(response(proofIndex))

  override fun awaitResponse(proofIndex: BlockIntervalProofIndex): SafeFuture<RollupProofResponseDto> =
    SafeFuture.completedFuture(requireNotNull(response(proofIndex)))

  private fun response(proofIndex: BlockIntervalProofIndex): RollupProofResponseDto? =
    if (responseProvider != null) responseProvider(proofIndex) else rollupProofResponseDto(proofIndex)

  private fun rollupProofResponseDto(
    proofIndex: BlockIntervalProofIndex,
  ): RollupProofResponseDto {
    return rollupProofResponseDto.copy(
      startBlockNumber = proofIndex.startBlockNumber.toLong(),
      publicInputs = RiscVProverClientTestFixtures.rollupProofPublicInputsDto(
        proofIndex.endBlockNumber.toLong(),
      ),
    )
  }
}
