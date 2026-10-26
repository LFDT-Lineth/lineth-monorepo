package lineth.coordinator.clients.prover

import linea.clients.ProverProofTransport
import linea.clients.RollupProofRequestV1
import linea.clients.RollupProofResponseV1
import linea.clients.RollupProverClientV1
import linea.crypto.HashFunction
import linea.crypto.Sha256HashFunction
import linea.domain.BlockIntervalProofIndex
import linea.kotlin.decodeHex
import linea.kotlin.encodeHex
import lineth.coordinator.clients.prover.BlockIntervalDto.Companion.toDto
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture

/**
 * Maps a [RollupProofRequestV1] domain request to the RISC-V rollup proof request DTO described by
 * `rollup_spec/src/rollup_spec/prover_io/schemas/getZkRollupProofV1.request.schema.json`.
 */
class RollupProofRequestDtoMapper(
  private val guestProgramId: String,
  private val provingSystem: String,
  private val chainId: Long,
  private val l2ExecutionProofProvider: ProofProvider<L2ExecutionProofResponseDto>? = null,
) : (RollupProofRequestV1) -> SafeFuture<FileBasedRollupProofRequestDto> {
  override fun invoke(request: RollupProofRequestV1): SafeFuture<FileBasedRollupProofRequestDto> {
    val l2ExecutionProofFutures = l2ExecutionProofProvider?.let {
      request.l2Executions.map { proofIndex ->
        l2ExecutionProofProvider.findProof(proofIndex).thenApply { response ->
          // proverVersion is response-only; the embedded l2ExecutionProof schema does not allow it.
          requireNotNull(response) {
            "L2 execution proof response was not found for proofIndex=$proofIndex"
          }.copy(proverVersion = null)
        }
      }
    } ?: emptyList()

    val l2ExecutionProofIndexes = if (l2ExecutionProofProvider == null) {
      request.l2Executions.map { it.toDto() }
    } else {
      null
    }

    return SafeFuture.collectAll(l2ExecutionProofFutures.stream())
      .thenApply { l2ExecutionProofResponseDtos ->
        FileBasedRollupProofRequestDto(
          guestProgramId = guestProgramId,
          provingSystem = provingSystem,
          proofRequest = RollupProofRequestParamsDto(
            chainId = chainId,
            conflations = request.conflations.map { it.toDto() },
            l2ExecutionProofs = l2ExecutionProofResponseDtos.ifEmpty { null },
            l2ExecutionProofIndexes = l2ExecutionProofIndexes,
            chunks = request.chunks.map { it.toDto() },
            parentDataRollingHash = request.parentDataRollingHash.encodeHex(),
            parentDataTailTakeBytes = request.parentDataTailTakeBytes,
            boundaryPrevDataRollingHash = request.boundaryPrevDataRollingHash?.encodeHex(),
          ),
          metadata = MetaDataDto(
            startBlockNumber = request.startBlockNumber.toLong(),
            endBlockNumber = request.endBlockNumber.toLong(),
            startBlockTimestamp = request.startBlockTimestamp.epochSeconds,
          ),
        )
      }
  }
}

/**
 * Maps the deserialized rollup proof response DTO onto the domain [RollupProofResponseV1] described by
 * `rollup_spec/src/rollup_spec/prover_io/schemas/getZkRollupProofV1.response.schema.json`.
 * The transport is responsible for parsing the JSON (read from a file or returned by a REST call) into
 * [RollupProofResponseDto] before this mapper runs.
 */
object RollupProofResponseDtoMapper : (
  RollupProofResponseDto,
) -> RollupProofResponseV1 {
  override fun invoke(
    responseDto: RollupProofResponseDto,
  ): RollupProofResponseV1 {
    return RollupProofResponseV1(
      startBlockNumber = responseDto.startBlockNumber.toULong(),
      endBlockNumber = responseDto.publicInputs.endBlockNumber.toULong(),
      proof = responseDto.proof.decodeHex(),
      publicInputs = responseDto.publicInputs.toDomainObject(),
      programVk = responseDto.programVk.decodeHex(),
    )
  }
}

typealias RollupProofTransport =
  ProverProofTransport<FileBasedRollupProofRequestDto, RollupProofResponseDto, BlockIntervalProofIndex>

/**
 * RISC-V rollup prover client. The request/response transport is injected via [transport], so the
 * same client works whether requests are written as JSON files or sent over REST.
 *
 * @param l2ExecutionProofProvider source of the l2-execution proofs inlined into each request; null
 *   sends only their indexes, for a prover (e.g. the prover gateway) that resolves them itself.
 */
class RollupProverClient(
  transport: RollupProofTransport,
  l2ExecutionProofProvider: ProofProvider<L2ExecutionProofResponseDto>?,
  guestProgramId: String,
  provingSystem: String,
  chainId: Long,
  proofRequestDtoMapper: (RollupProofRequestV1) -> SafeFuture<FileBasedRollupProofRequestDto> =
    RollupProofRequestDtoMapper(
      guestProgramId,
      provingSystem,
      chainId,
      l2ExecutionProofProvider,
    ),
  proofResponseDtoMapper: (RollupProofResponseDto) -> RollupProofResponseV1 =
    RollupProofResponseDtoMapper,
  hashFunction: HashFunction = Sha256HashFunction(),
  log: Logger = LOG,
) : GenericProverClient<
  RollupProofRequestV1,
  RollupProofResponseV1,
  FileBasedRollupProofRequestDto,
  RollupProofResponseDto,
  BlockIntervalProofIndex,
  >(
  transport = transport,
  proofIndexProvider = BlockIntervalProofIndexProvider<RollupProofRequestV1>(hashFunction),
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
