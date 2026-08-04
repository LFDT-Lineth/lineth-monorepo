package linea.coordinator.clients.prover.riscv

import linea.clients.ProverProofTransport
import linea.clients.RollupProofRequestV1
import linea.clients.RollupProofResponseV1
import linea.clients.RollupProverClientV1
import linea.crypto.HashFunction
import linea.crypto.Sha256HashFunction
import linea.domain.BlockIntervalProofIndex
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
    val l2ExecutionProofFutures = request.l2Executions.map { proofIndex ->
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
            l2ExecutionProofs = l2ExecutionProofResponseDtos.mapIndexed { index, response ->
              val proofIndex = request.l2Executions[index]
              val proofResponse = requireNotNull(response) {
                "L2 execution proof response was not found for proofIndex=$proofIndex"
              }
              L2ExecutionProofDto(
                proof = proofResponse.proof,
                startBlockNumber = proofResponse.startBlockNumber,
                publicInputs = proofResponse.publicInputs,
                l2L1Messages = proofResponse.l2L1Messages,
                txFroms = proofResponse.txFroms,
                filteredAddresses = proofResponse.filteredAddresses,
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
        l2ExecutionProofIndexes = request.l2Executions,
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
    )
  }
}

internal class RollupProofIndexProvider(
  private val hashFunction: HashFunction,
) : (RollupProofRequestV1) -> BlockIntervalProofIndex {
  override fun invoke(request: RollupProofRequestV1): BlockIntervalProofIndex {
    val content = request.toString().toByteArray()
    return BlockIntervalProofIndex(
      startBlockNumber = request.startBlockNumber,
      endBlockNumber = request.endBlockNumber,
      hash = hashFunction.hash(content),
      startBlockTimestamp = request.startBlockTimestamp,
    )
  }
}

typealias FileBasedRollupProofTransport =
  ProverProofTransport<FileBasedRollupProofRequestDto, RollupProofResponseDto, BlockIntervalProofIndex>

private typealias RestfulRollupProofTransport =
  ProverProofTransport<RestfulRollupProofRequestDto, RollupProofResponseDto, BlockIntervalProofIndex>

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
  proofResponseDtoMapper: (RollupProofResponseDto) -> RollupProofResponseV1 =
    RollupProofResponseDtoMapper,
  hashFunction: HashFunction = Sha256HashFunction(),
  log: Logger = LOG,
) : GenericRiscVProverClient<
  RollupProofRequestV1,
  RollupProofResponseV1,
  FileBasedRollupProofRequestDto,
  RollupProofResponseDto,
  BlockIntervalProofIndex,
  >(
  transport = transport,
  proofIndexProvider = RollupProofIndexProvider(hashFunction),
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
  proofResponseDtoMapper: (RollupProofResponseDto) -> RollupProofResponseV1 =
    RollupProofResponseDtoMapper,
  hashFunction: HashFunction = Sha256HashFunction(),
  log: Logger = LOG,
) : GenericRiscVProverClient<
  RollupProofRequestV1,
  RollupProofResponseV1,
  RestfulRollupProofRequestDto,
  RollupProofResponseDto,
  BlockIntervalProofIndex,
  >(
  transport = transport,
  proofIndexProvider = RollupProofIndexProvider(hashFunction),
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
