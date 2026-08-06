package linea.coordinator.clients.prover.riscv

import linea.clients.ProverProofTransport
import linea.clients.RollupAggregationProofRequestV1
import linea.clients.RollupAggregationProofResponseV1
import linea.clients.RollupAggregationProverClientV1
import linea.crypto.HashFunction
import linea.crypto.Sha256HashFunction
import linea.domain.BlockIntervalProofIndex
import linea.kotlin.decodeHex
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture

/**
 * Maps a [RollupAggregationProofRequestV1] domain request to the RISC-V rollup-aggregation proof request DTO described by
 * `rollup_spec/prover_io/schemas/getZkRollupAggregationProofV1.request.schema.json`.
 */
internal class FileBasedRollupAggregationProofRequestDtoMapper(
  private val guestProgramId: String,
  private val rollupProofTransport: FileBasedRollupProofTransport,
) : (RollupAggregationProofRequestV1) -> SafeFuture<FileBasedRollupAggregationProofRequestDto> {
  override fun invoke(request: RollupAggregationProofRequestV1): SafeFuture<FileBasedRollupAggregationProofRequestDto> {
    val rollupProofFutures = request.rollupProofs.map { proofIndex ->
      rollupProofTransport.findResponse(proofIndex)
    }
    return SafeFuture.collectAll(rollupProofFutures.stream())
      .thenApply { rollupResponseDtos ->
        FileBasedRollupAggregationProofRequestDto(
          guestProgramId = guestProgramId,
          proofRequest = FileBasedRollupAggregationProofRequestParamsDto(
            rollupProofs = rollupResponseDtos.map { it ->
              RollupProofDto(
                proof = it!!.proof,
                startBlockNumber = it.startBlockNumber,
                publicInputs = it.publicInputs,
                l2L1Roots = it.l2L1Roots,
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
 * Maps a [RollupAggregationProofRequestV1] domain request to the RISC-V rollup proof request DTO described by
 * `rollup_spec/prover_io/schemas/getZkRollupAggregationProofV1.request.schema.json` with proof index to reference
 * each rollup proof response.
 */
internal class RestfulRollupAggregationProofRequestDtoMapper(
  private val guestProgramId: String,
) : (RollupAggregationProofRequestV1) -> SafeFuture<RestfulRollupAggregationProofRequestDto> {
  override fun invoke(request: RollupAggregationProofRequestV1): SafeFuture<RestfulRollupAggregationProofRequestDto> {
    val dto = RestfulRollupAggregationProofRequestDto(
      guestProgramId = guestProgramId,
      proofRequest = RestfulRollupAggregationProofRequestParamsDto(
        rollupProofIndexes = request.rollupProofs,
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
 * Maps the deserialized rollup-aggregation proof response DTO onto the domain [RollupAggregationProofResponseV1]
 * described by `rollup_spec/prover_io/schemas/getZkRollupAggregationProofV1.response.schema.json`.
 * The transport is responsible for parsing the JSON (read from a file or returned by a REST call) into
 * [RollupAggregationProofResponseDto] before this mapper runs.
 */
internal object RollupAggregationProofResponseDtoMapper :
  (RollupAggregationProofResponseDto) -> RollupAggregationProofResponseV1 {
  override fun invoke(
    responseDto: RollupAggregationProofResponseDto,
  ): RollupAggregationProofResponseV1 {
    return RollupAggregationProofResponseV1(
      startBlockNumber = responseDto.startBlockNumber.toULong(),
      endBlockNumber = responseDto.publicInputs.endBlockNumber.toULong(),
      proof = responseDto.proof.decodeHex(),
      publicInputs = responseDto.publicInputs.toDomainObject(),
      l2L1Roots = responseDto.l2L1Roots.map { it.decodeHex() },
      filteredAddresses = responseDto.filteredAddresses.map { it.decodeHex() },
      l2MessagingBlocksOffsets = responseDto.l2MessagingBlocksOffsets.map { it.toULong() },
    )
  }
}

private typealias FileBasedRollupAggregationProofTransport =
  ProverProofTransport<
    FileBasedRollupAggregationProofRequestDto,
    RollupAggregationProofResponseDto,
    BlockIntervalProofIndex,
    >

private typealias RestfulRollupAggregationProofTransport =
  ProverProofTransport<
    RestfulRollupAggregationProofRequestDto,
    RollupAggregationProofResponseDto,
    BlockIntervalProofIndex,
    >

/**
 * RISC-V file-based rollup-aggregation prover client.
 * The request/response transport is injected via file-based transport.
 */
class FileBasedRollupAggregationProverClient(
  transport: FileBasedRollupAggregationProofTransport,
  rollupProofTransport: FileBasedRollupProofTransport,
  guestProgramId: String,
  proofRequestDtoMapper: (RollupAggregationProofRequestV1)
  -> SafeFuture<FileBasedRollupAggregationProofRequestDto> = FileBasedRollupAggregationProofRequestDtoMapper(
    guestProgramId,
    rollupProofTransport,
  ),
  proofResponseDtoMapper: (RollupAggregationProofResponseDto)
  -> RollupAggregationProofResponseV1 = RollupAggregationProofResponseDtoMapper,
  hashFunction: HashFunction = Sha256HashFunction(),
  log: Logger = LOG,
) : GenericRiscVProverClient<
  RollupAggregationProofRequestV1,
  RollupAggregationProofResponseV1,
  FileBasedRollupAggregationProofRequestDto,
  RollupAggregationProofResponseDto,
  BlockIntervalProofIndex,
  >(
  transport = transport,
  proofIndexProvider = BlockIntervalProofIndexProvider<RollupAggregationProofRequestV1>(hashFunction),
  requestMapper = proofRequestDtoMapper,
  responseMapper = proofResponseDtoMapper,
  proofTypeLabel = "rollup-aggregation",
  log = log,
),
  RollupAggregationProverClientV1 {

  companion object {
    val LOG: Logger = LogManager.getLogger(FileBasedRollupAggregationProverClient::class.java)
  }
}

/**
 * RISC-V Restful rollup-aggregation prover client.
 * The request/response transport is injected via Restful transport.
 */
class RestfulRollupAggregationProverClient(
  transport: RestfulRollupAggregationProofTransport,
  guestProgramId: String,
  proofRequestDtoMapper: (RollupAggregationProofRequestV1) -> SafeFuture<RestfulRollupAggregationProofRequestDto> =
    RestfulRollupAggregationProofRequestDtoMapper(guestProgramId),
  proofResponseDtoMapper: (RollupAggregationProofResponseDto)
  -> RollupAggregationProofResponseV1 = RollupAggregationProofResponseDtoMapper,
  hashFunction: HashFunction = Sha256HashFunction(),
  log: Logger = LOG,
) : GenericRiscVProverClient<
  RollupAggregationProofRequestV1,
  RollupAggregationProofResponseV1,
  RestfulRollupAggregationProofRequestDto,
  RollupAggregationProofResponseDto,
  BlockIntervalProofIndex,
  >(
  transport = transport,
  proofIndexProvider = BlockIntervalProofIndexProvider<RollupAggregationProofRequestV1>(hashFunction),
  requestMapper = proofRequestDtoMapper,
  responseMapper = proofResponseDtoMapper,
  proofTypeLabel = "rollup-aggregation",
  log = log,
),
  RollupAggregationProverClientV1 {

  companion object {
    val LOG: Logger = LogManager.getLogger(RestfulRollupAggregationProverClient::class.java)
  }
}
