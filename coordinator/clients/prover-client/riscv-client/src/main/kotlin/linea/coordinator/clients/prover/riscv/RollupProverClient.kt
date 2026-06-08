package linea.coordinator.clients.prover.riscv

import linea.clients.RollupProofRequestV1
import linea.clients.RollupProofResponse
import linea.clients.RollupProverClientV1
import linea.domain.CompressionProofIndex
import linea.kotlin.decodeHex
import linea.kotlin.encodeHex
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture

/**
 * Maps a [RollupProofRequestV1] domain request to the RISC-V rollup proof request DTO described by
 * `rollup_spec/prover_inputs/getZkRollupProof.request.json` (underscore-prefixed documentation fields skipped).
 *
 * NOTE: the legacy [RollupProofRequestV1] does not carry the per-blob KZG inputs in the exact RISC-V form,
 * the inlined l2-execution proofs, nor the 14-field public-inputs tuple. Those are flagged with `TODO` until the
 * upstream request plumbing provides them.
 */
internal class RollupProofRequestDtoMapper(
  private val proverVersion: String,
  private val chainId: Long,
) : (RollupProofRequestV1) -> SafeFuture<RollupProofRequestDto> {
  override fun invoke(request: RollupProofRequestV1): SafeFuture<RollupProofRequestDto> {
    val dto = RollupProofRequestDto(
      proverVersion = proverVersion,
      chainId = chainId,
      blockRange = BlockRangeDto(
        startBlockNumber = request.startBlockNumber.toLong(),
        endBlockNumber = request.endBlockNumber.toLong(),
      ),
      blobs = emptyList(), // TODO: RollupProofRequestV1 should contain blobs info
      shnarfTransition = ShnarfTransitionDto(
        parentShnarf = request.parentShnarf.encodeHex(),
        endShnarf = request.endShnarf.encodeHex(),
      ),
      l2ExecutionProofs = request.l2ExecutionProofs.map { it.fromDomainObject() },
    )

    return SafeFuture.completedFuture(dto)
  }
}

/**
 * Maps the deserialized rollup proof response DTO onto the domain [RollupProofResponse]. Field names and types are
 * identical between the two, so this is a straight field copy. The transport is responsible for parsing the JSON
 * (read from a file or returned by a REST call) into [RollupProofResponseDto] before this mapper runs.
 */
internal object RollupProofResponseDtoMapper : (CompressionProofIndex, RollupProofResponseDto) -> RollupProofResponse {
  override fun invoke(
    proofIndex: CompressionProofIndex,
    responseDto: RollupProofResponseDto,
  ): RollupProofResponse {
    return RollupProofResponse(
      startBlockNumber = responseDto.startBlockNumber.toULong(),
      endBlockNumber = responseDto.endBlockNumber.toULong(),
      proof = responseDto.proof.decodeHex(),
      publicInputs = responseDto.publicInputs.toDomainObject(),
      L2L1Roots = responseDto.L2L1Roots.map { it.decodeHex() },
      filteredAddresses = responseDto.filteredAddresses.map { it.decodeHex() },
    )
  }
}

private typealias RollupProofTransport =
  ProverProofTransport<RollupProofRequestDto, RollupProofResponseDto, CompressionProofIndex>

/**
 * RISC-V rollup prover client. The request/response transport is injected via [transport], so the same client works
 * whether requests are written as JSON files or sent over REST.
 */
class RollupProverClient(
  private val transport: RollupProofTransport,
  proverVersion: String,
  chainId: Long,
  proofRequestDtoMapper: (RollupProofRequestV1) -> SafeFuture<RollupProofRequestDto> =
    RollupProofRequestDtoMapper(proverVersion, chainId),
  proofResponseDtoMapper: (CompressionProofIndex, RollupProofResponseDto) -> RollupProofResponse =
    RollupProofResponseDtoMapper,
  log: Logger = LOG,
) : GenericRiscVProverClient<
  RollupProofRequestV1,
  RollupProofResponse,
  RollupProofRequestDto,
  RollupProofResponseDto,
  CompressionProofIndex,
  >(
  transport = transport,
  proofIndexProvider = { request ->
    CompressionProofIndex(
      startBlockNumber = request.startBlockNumber,
      endBlockNumber = request.endBlockNumber,
      hash = request.endShnarf,
      startBlockTimestamp = request.startBlockTimestamp,
    )
  },
  requestMapper = proofRequestDtoMapper,
  responseMapper = proofResponseDtoMapper,
  proofTypeLabel = "rollup",
  log = log,
),
  RollupProverClientV1 {

  companion object {
    val LOG: Logger = LogManager.getLogger(RollupProverClient::class.java)
  }
}
