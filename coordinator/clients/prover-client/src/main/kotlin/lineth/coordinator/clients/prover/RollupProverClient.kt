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
 * `rollup_spec/prover_io/schemas/getZkRollupProofV1.request.schema.json`.
 */
class RollupProofRequestDtoMapper(
  private val programId: String,
  private val provingSystemVersion: String,
  private val chainId: Long,
  private val l2ExecutionProofProvider: ProofProvider<L2ExecutionProofResponseDto>? = null,
) : (RollupProofRequestV1) -> SafeFuture<FileBasedRollupProofRequestDto> {
  override fun invoke(request: RollupProofRequestV1): SafeFuture<FileBasedRollupProofRequestDto> {
    val l2ExecutionProofFutures = l2ExecutionProofProvider?.let {
      request.l2Executions.map { proofIndex ->
        l2ExecutionProofProvider.findProof(proofIndex).thenApply { response ->
          requireNotNull(response) {
            "L2 execution proof response was not found for proofIndex=$proofIndex"
          }
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
          programId = programId,
          provingSystemVersion = provingSystemVersion,
          proofRequest = RollupProofRequestParamsDto(
            chainId = chainId,
            conflations = request.conflations.map { it.toDto() },
            l2ExecutionProofs = l2ExecutionProofResponseDtos.ifEmpty { null },
            l2ExecutionProofIndexes = l2ExecutionProofIndexes,
            chunks = request.chunks.map { it.encodeHex() },
            parentDataRollingHash = request.parentDataRollingHash.encodeHex(),
            startOffset = request.startOffset,
            opaquePrefixBytes = request.opaquePrefixBytes.takeIf { it.isNotEmpty() }?.encodeHex(),
            opaqueSuffixBytes = request.opaqueSuffixBytes.takeIf { it.isNotEmpty() }?.encodeHex(),
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
 * `rollup_spec/prover_io/schemas/getZkRollupProofV1.response.schema.json`.
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
      l2L1Roots = responseDto.l2L1Roots.map { it.decodeHex() },
      filteredAddresses = responseDto.filteredAddresses.map { it.decodeHex() },
      programVk = responseDto.programVk.decodeHex(),
    )
  }
}

typealias FileBasedRollupProofTransport =
  ProverProofTransport<FileBasedRollupProofRequestDto, RollupProofResponseDto, BlockIntervalProofIndex>

/**
 * RISC-V file-based rollup prover client.
 * The request/response transport is injected via file-based transport.
 */
class FileBasedRollupProverClient(
  transport: FileBasedRollupProofTransport,
  l2ExecutionProofTransport: L2ExecutionProofTransport,
  programId: String,
  provingSystemVersion: String,
  chainId: Long,
  proofRequestDtoMapper: (RollupProofRequestV1) -> SafeFuture<FileBasedRollupProofRequestDto> =
    RollupProofRequestDtoMapper(
      programId,
      provingSystemVersion,
      chainId,
      l2ExecutionProofTransport::findResponse,
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
    val LOG: Logger = LogManager.getLogger(FileBasedRollupProverClient::class.java)
  }
}
