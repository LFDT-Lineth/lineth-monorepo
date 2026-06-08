package linea.coordinator.clients.prover.riscv

import linea.clients.RollupAggregationProofRequestV1
import linea.clients.RollupAggregationProofResponse
import linea.clients.RollupAggregationProverClientV1
import linea.crypto.HashFunction
import linea.crypto.Sha256HashFunction
import linea.domain.AggregationProofIndex
import linea.kotlin.decodeHex
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture

/**
 * Maps a [RollupAggregationProofRequestV1] domain request to the RISC-V rollup-aggregation proof request DTO described by
 * `rollup_spec/prover_inputs/getZkRollupAggregationProof.request.json` (underscore-prefixed documentation fields
 * skipped).
 *
 * NOTE: the legacy [RollupAggregationProofRequestV1] references the proofs to aggregate indirectly (by index / file name) rather
 * than inlining the recursive STARK proofs and their public inputs the RISC-V schema expects. The inlined
 * `rollupProofs` and the expected 14-field public-inputs tuple are flagged with `TODO` until that data is available.
 */
internal class RollupAggregationProofRequestDtoMapper(
  private val proverVersion: String,
  private val chainId: Long,
) : (RollupAggregationProofRequestV1) -> SafeFuture<RollupAggregationProofRequestDto> {
  override fun invoke(request: RollupAggregationProofRequestV1): SafeFuture<RollupAggregationProofRequestDto> {
    val dto = RollupAggregationProofRequestDto(
      proverVersion = proverVersion,
      chainId = chainId,
      blockRange = BlockRangeDto(
        startBlockNumber = request.startBlockNumber.toLong(),
        endBlockNumber = request.endBlockNumber.toLong(),
      ),
      rollupProofs = request.rollupProofs.map { it.fromDomainObject() },
    )

    return SafeFuture.completedFuture(dto)
  }
}

/**
 * Maps the deserialized rollup-aggregation proof response DTO onto the domain [RollupAggregationProofResponse]. Field
 * names and types are identical between the two, so this is a straight field copy. The transport is responsible for
 * parsing the JSON (read from a file or returned by a REST call) into [RollupAggregationProofResponseDto] before this
 * mapper runs.
 */
internal object RollupAggregationProofResponseDtoMapper :
  (AggregationProofIndex, RollupAggregationProofResponseDto) -> RollupAggregationProofResponse {
  override fun invoke(
    proofIndex: AggregationProofIndex,
    responseDto: RollupAggregationProofResponseDto,
  ): RollupAggregationProofResponse {
    return RollupAggregationProofResponse(
      startBlockNumber = responseDto.startBlockNumber.toULong(),
      endBlockNumber = responseDto.endBlockNumber.toULong(),
      proof = responseDto.proof.decodeHex(),
      publicInputs = responseDto.publicInputs.toDomainObject(),
    )
  }
}

private typealias RollupAggregationProofTransport =
  ProverProofTransport<RollupAggregationProofRequestDto, RollupAggregationProofResponseDto, AggregationProofIndex>

/**
 * RISC-V rollup-aggregation prover client. The request/response transport is injected via [transport], so the same
 * client works whether requests are written as JSON files or sent over REST.
 */
class RollupAggregationProverClient(
  private val transport: RollupAggregationProofTransport,
  proverVersion: String,
  chainId: Long,
  hashFunction: HashFunction = Sha256HashFunction(),
  proofRequestDtoMapper: (RollupAggregationProofRequestV1) -> SafeFuture<RollupAggregationProofRequestDto> =
    RollupAggregationProofRequestDtoMapper(proverVersion, chainId),
  proofResponseDtoMapper: (AggregationProofIndex, RollupAggregationProofResponseDto) -> RollupAggregationProofResponse =
    RollupAggregationProofResponseDtoMapper,
  log: Logger = LOG,
) : GenericRiscVProverClient<
  RollupAggregationProofRequestV1,
  RollupAggregationProofResponse,
  RollupAggregationProofRequestDto,
  RollupAggregationProofResponseDto,
  AggregationProofIndex,
  >(
  transport = transport,
  proofIndexProvider = createProofIndexProviderFn(hashFunction),
  requestMapper = proofRequestDtoMapper,
  responseMapper = proofResponseDtoMapper,
  proofTypeLabel = "rollup-aggregation",
  log = log,
),
  RollupAggregationProverClientV1 {

  companion object {
    val LOG: Logger = LogManager.getLogger(RollupAggregationProverClient::class.java)

    /**
     * Builds the proof-index provider. The aggregation proof index requires a content hash; this draft hashes a
     * deterministic projection of the request (block range) so that identical requests map to the same index.
     *
     * TODO: extend the hashed content to cover the full inlined rollup-proof set once those fields are available, so
     *  the index is collision-resistant across aggregations sharing a block range.
     */
    fun createProofIndexProviderFn(
      hashFunction: HashFunction,
    ): (RollupAggregationProofRequestV1) -> AggregationProofIndex {
      return { request ->
        val content = "${request.startBlockNumber}-${request.endBlockNumber}".toByteArray()
        AggregationProofIndex(
          startBlockNumber = request.startBlockNumber,
          endBlockNumber = request.endBlockNumber,
          hash = hashFunction.hash(content),
          startBlockTimestamp = request.startBlockTimestamp,
        )
      }
    }
  }
}
