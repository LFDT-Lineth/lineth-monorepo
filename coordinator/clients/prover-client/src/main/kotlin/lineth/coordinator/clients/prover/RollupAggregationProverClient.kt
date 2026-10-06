package lineth.coordinator.clients.prover

import linea.clients.ProverProofTransport
import linea.clients.RollupAggregationProofRequestV1
import linea.clients.RollupAggregationProofResponseV1
import linea.clients.RollupAggregationProverClientV1
import linea.crypto.HashFunction
import linea.crypto.Sha256HashFunction
import linea.domain.BlockIntervalProofIndex
import linea.kotlin.decodeHex
import lineth.coordinator.clients.prover.BlockIntervalDto.Companion.toDto
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture

/**
 * Maps a [RollupAggregationProofRequestV1] domain request to the RISC-V rollup-aggregation proof request DTO
 * described by `rollup_spec/prover_io/schemas/getZkRollupAggregationProofV1.request.schema.json`.
 *
 * When rollupProofProvider is null, it will provide only rollup indexes and prover gateway will fill in the proof
 */
class RollupAggregationProofRequestDtoMapper(
  private val programId: String,
  private val provingSystemVersion: String,
  private val rollupProofProvider: ProofProvider<RollupProofResponseDto>?,
) : (RollupAggregationProofRequestV1) -> SafeFuture<FileBasedRollupAggregationProofRequestDto> {

  override fun invoke(request: RollupAggregationProofRequestV1): SafeFuture<FileBasedRollupAggregationProofRequestDto> {
    val rollupProofFutures = rollupProofProvider?.let {
      request.rollupProofs.map { proofIndex ->
        rollupProofProvider.findProof(proofIndex)
          .thenApply { response ->
            requireNotNull(response) {
              "Rollup proof response was not found for proofIndex=$proofIndex"
            }
          }
      }
    } ?: emptyList()
    val rollupProofsIndexes = if (rollupProofProvider == null) {
      request.rollupProofs.map { it.toDto() }
    } else {
      null
    }

    return SafeFuture
      .collectAll(rollupProofFutures.stream())
      .thenApply { rollupResponseDtos ->
        FileBasedRollupAggregationProofRequestDto(
          programId = programId,
          provingSystemVersion = provingSystemVersion,
          proofRequest = FileBasedRollupAggregationProofRequestParamsDto(
            rollupProofs = rollupResponseDtos.ifEmpty { null },
            rollupProofsIndexes = rollupProofsIndexes,
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
 * Maps the deserialized rollup-aggregation proof response DTO onto the domain [RollupAggregationProofResponseV1]
 * described by `rollup_spec/prover_io/schemas/getZkRollupAggregationProofV1.response.schema.json`.
 * The transport is responsible for parsing the JSON (read from a file or returned by a REST call) into
 * [RollupAggregationProofResponseDto] before this mapper runs.
 */
object RollupAggregationProofResponseDtoMapper :
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

typealias RollupAggregationProofTransport =
  ProverProofTransport<
    FileBasedRollupAggregationProofRequestDto,
    RollupAggregationProofResponseDto,
    BlockIntervalProofIndex,
    >

/**
 * RISC-V rollup-aggregation prover client. The request/response transport is injected via
 * [transport], so the same client works whether requests are written as JSON files or sent over REST.
 *
 * @param rollupProofProvider source of the rollup proofs inlined into each request; null sends only
 *   their indexes, for a prover (e.g. the prover gateway) that resolves them itself.
 */
class RollupAggregationProverClient(
  transport: RollupAggregationProofTransport,
  rollupProofProvider: ProofProvider<RollupProofResponseDto>?,
  programId: String,
  provingSystemVersion: String,
  proofRequestDtoMapper: (RollupAggregationProofRequestV1)
  -> SafeFuture<FileBasedRollupAggregationProofRequestDto> = RollupAggregationProofRequestDtoMapper(
    programId,
    provingSystemVersion,
    rollupProofProvider,
  ),
  proofResponseDtoMapper: (RollupAggregationProofResponseDto)
  -> RollupAggregationProofResponseV1 = RollupAggregationProofResponseDtoMapper,
  hashFunction: HashFunction = Sha256HashFunction(),
  log: Logger = LOG,
) : GenericProverClient<
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
    val LOG: Logger = LogManager.getLogger(RollupAggregationProverClient::class.java)
  }
}
