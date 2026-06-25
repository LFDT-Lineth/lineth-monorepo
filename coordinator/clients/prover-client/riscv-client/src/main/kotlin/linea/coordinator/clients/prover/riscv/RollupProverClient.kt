package linea.coordinator.clients.prover.riscv

import linea.clients.ProverProofTransport
import linea.clients.RollupProofRequestV1
import linea.clients.RollupProofResponseV1
import linea.clients.RollupProverClientV1
import linea.domain.CompressionProofIndex
import linea.kotlin.decodeHex
import linea.kotlin.encodeHex
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture

/**
 * Maps a [RollupProofRequestV1] domain request to the RISC-V rollup proof request DTO described by
 * `rollup_spec/prover_io/schemas/getZkRollupProofV1.request.schema.json`.
 */
internal class FileBasedRollupProofRequestDtoMapper(
  private val guestProgramId: String,
  private val chainId: Long,
  private val l2ExecutionProofTransport: L2ExecutionProofTransport,
) : (RollupProofRequestV1) -> SafeFuture<FileBasedRollupProofRequestDto> {
  override fun invoke(request: RollupProofRequestV1): SafeFuture<FileBasedRollupProofRequestDto> {
    val l2ExecutionProofFutures = request.l2ExecutionProofIndexes.map { proofIndex ->
      l2ExecutionProofTransport.findResponse(proofIndex)
    }
    return SafeFuture.collectAll(l2ExecutionProofFutures.stream())
      .thenApply { l2ExecutionProofResponseDtos ->
        FileBasedRollupProofRequestDto(
          guestProgramId = guestProgramId,
          proofRequest = FileBasedRollupProofRequestParamsDto(
            chainId = chainId,
            blobs = request.blobs.map { it.fromDomainObject() },
            parentShnarf = request.parentShnarf.encodeHex(),
            l2ExecutionProofs = l2ExecutionProofResponseDtos.map { it ->
              L2ExecutionProofDto(
                proof = it!!.proof,
                startBlockNumber = it.startBlockNumber,
                endBlockNumber = it.publicInputs.endBlockNumber,
                publicInputs = it.publicInputs,
                l2L1Messages = it.l2L1Messages,
                txFroms = it.txFroms,
                filteredAddresses = it.filteredAddresses,
              )
            },
          ),
          metadata = MetaDataDto(
            startBlockNumber = request.startBlockNumber.toLong(),
            endBlockNumber = request.endBlockNumber.toLong(),
          ),
        )
      }
  }
}

/**
 * Maps a [RollupProofRequestV1] domain request to the RISC-V rollup proof request DTO described by
 * `rollup_spec/prover_io/schemas/getZkRollupProofV1.request.schema.json` with proof index to reference
 * each l2-execution proof response.
 */
internal class RestfulRollupProofRequestDtoMapper(
  private val guestProgramId: String,
  private val chainId: Long,
) : (RollupProofRequestV1) -> SafeFuture<RestfulRollupProofRequestDto> {
  override fun invoke(request: RollupProofRequestV1): SafeFuture<RestfulRollupProofRequestDto> {
    val dto = RestfulRollupProofRequestDto(
      guestProgramId = guestProgramId,
      proofRequest = RestfulRollupProofRequestParamsDto(
        chainId = chainId,
        blobs = request.blobs.map { it.fromDomainObject() },
        parentShnarf = request.parentShnarf.encodeHex(),
        l2ExecutionProofIndexes = request.l2ExecutionProofIndexes,
      ),
      metadata = MetaDataDto(
        startBlockNumber = request.startBlockNumber.toLong(),
        endBlockNumber = request.endBlockNumber.toLong(),
      ),
    )
    return SafeFuture.completedFuture(dto)
  }
}

/**
 * Maps the deserialized rollup proof response DTO onto the domain [RollupProofResponseV1] described by
 * `rollup_spec/prover_io/schemas/getZkRollupProofV1.response.schema.json`.
 * The transport is responsible for parsing the JSON (read from a file or returned by a REST call) into
 * [RollupProofResponseDto] before this mapper runs.
 */
internal object RollupProofResponseDtoMapper : (
  CompressionProofIndex,
  RollupProofResponseDto,
) -> RollupProofResponseV1 {
  override fun invoke(
    proofIndex: CompressionProofIndex,
    responseDto: RollupProofResponseDto,
  ): RollupProofResponseV1 {
    return RollupProofResponseV1(
      proverVersion = responseDto.proverVersion,
      startBlockNumber = responseDto.startBlockNumber.toULong(),
      endBlockNumber = responseDto.publicInputs.endBlockNumber.toULong(),
      proof = responseDto.proof.decodeHex(),
      publicInputs = responseDto.publicInputs.toDomainObject(),
      l2L1Roots = responseDto.l2L1Roots.map { it.decodeHex() },
      filteredAddresses = responseDto.filteredAddresses.map { it.decodeHex() },
    )
  }
}

internal object RollupProofIndexProvider : (RollupProofRequestV1) -> CompressionProofIndex {
  override fun invoke(request: RollupProofRequestV1): CompressionProofIndex {
    return CompressionProofIndex(
      startBlockNumber = request.startBlockNumber,
      endBlockNumber = request.endBlockNumber,
      hash = request.endShnarf,
      startBlockTimestamp = request.startBlockTimestamp,
    )
  }
}

typealias FileBasedRollupProofTransport =
  ProverProofTransport<FileBasedRollupProofRequestDto, RollupProofResponseDto, CompressionProofIndex>

private typealias RestfulRollupProofTransport =
  ProverProofTransport<RestfulRollupProofRequestDto, RollupProofResponseDto, CompressionProofIndex>

/**
 * RISC-V file-based rollup prover client.
 * The request/response transport is injected via file-based transport.
 */
class FileBasedRollupProverClient(
  transport: FileBasedRollupProofTransport,
  l2ExecutionProofTransport: L2ExecutionProofTransport,
  guestProgramId: String,
  chainId: Long,
  proofRequestDtoMapper: (RollupProofRequestV1) -> SafeFuture<FileBasedRollupProofRequestDto> =
    FileBasedRollupProofRequestDtoMapper(guestProgramId, chainId, l2ExecutionProofTransport),
  proofResponseDtoMapper: (CompressionProofIndex, RollupProofResponseDto) -> RollupProofResponseV1 =
    RollupProofResponseDtoMapper,
  log: Logger = LOG,
) : GenericRiscVProverClient<
  RollupProofRequestV1,
  RollupProofResponseV1,
  FileBasedRollupProofRequestDto,
  RollupProofResponseDto,
  CompressionProofIndex,
  >(
  transport = transport,
  proofIndexProvider = RollupProofIndexProvider,
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

/**
 * RISC-V Restful rollup prover client.
 * The request/response transport is injected via Restful transport.
 */
class RestfulRollupProverClient(
  transport: RestfulRollupProofTransport,
  guestProgramId: String,
  chainId: Long,
  proofRequestDtoMapper: (RollupProofRequestV1) -> SafeFuture<RestfulRollupProofRequestDto> =
    RestfulRollupProofRequestDtoMapper(guestProgramId, chainId),
  proofResponseDtoMapper: (CompressionProofIndex, RollupProofResponseDto) -> RollupProofResponseV1 =
    RollupProofResponseDtoMapper,
  log: Logger = LOG,
) : GenericRiscVProverClient<
  RollupProofRequestV1,
  RollupProofResponseV1,
  RestfulRollupProofRequestDto,
  RollupProofResponseDto,
  CompressionProofIndex,
  >(
  transport = transport,
  proofIndexProvider = RollupProofIndexProvider,
  requestMapper = proofRequestDtoMapper,
  responseMapper = proofResponseDtoMapper,
  proofTypeLabel = "rollup",
  log = log,
),
  RollupProverClientV1 {

  companion object {
    val LOG: Logger = LogManager.getLogger(RestfulRollupProverClient::class.java)
  }
}
