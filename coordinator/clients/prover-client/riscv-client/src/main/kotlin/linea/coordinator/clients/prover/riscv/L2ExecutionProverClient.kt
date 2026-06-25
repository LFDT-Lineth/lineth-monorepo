package linea.coordinator.clients.prover.riscv

import linea.clients.L2ExecutionProofRequestV1
import linea.clients.L2ExecutionProofResponseV1
import linea.clients.L2ExecutionProverClientV1
import linea.clients.ProverProofTransport
import linea.domain.ExecutionProofIndex
import linea.kotlin.decodeHex
import linea.kotlin.encodeHex
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture

/**
 * Maps a [L2ExecutionProofRequestV1] domain request to the RISC-V l2-execution proof request DTO described by
 * `rollup_spec/prover_io/schemas/getZkL2ExecutionProofV1.request.schema.json`.
 */
internal class L2ExecutionProofRequestDtoMapper(
  private val guestProgramId: String,
  private val chainConfig: ChainConfigDto,
) : (L2ExecutionProofRequestV1) -> SafeFuture<L2ExecutionProofRequestDto> {
  override fun invoke(request: L2ExecutionProofRequestV1): SafeFuture<L2ExecutionProofRequestDto> {
    val payloads = request.executionPayloads.map { executionPayload ->
      val blockNumber = executionPayload.blockNumber
      val executionWitness = request.executionWitnesses.find { it.blockNumber == blockNumber }
      val executionRequests = request.executionRequests.find { it.blockNumber == blockNumber }
      val forcedTransactions = request.forcedTransactions.filter { it -> it.blockNumber == blockNumber }
      val statelessInputDto = StatelessInputDto(
        newPayloadRequest = NewPayloadRequestDto(
          executionPayload = executionPayload.fromDomainObject(),
          versionedHashes = emptyList(),
          parentBeaconBlockRoot = ByteArray(32).encodeHex(),
          executionRequests = executionRequests?.executionRequests?.map { it.encodeHex() } ?: emptyList(),
        ),
        executionWitness = executionWitness!!.fromDomainObject(),
      )
      PayloadInputDto(
        statelessInput = statelessInputDto,
        rollupExtension = RollupExtensionDto(
          forcedTransactions = forcedTransactions.map { it.fromDomainObject() },
        ),
      )
    }

    val dto = L2ExecutionProofRequestDto(
      guestProgramId = guestProgramId,
      proofRequest = L2ExecutionProofRequestParamsDto(
        parentFtxRollingHash = request.parentFtxRollingHash.encodeHex(),
        parentLastProcessedFtxNumber = request.parentFtxNumber.toLong(),
        chainConfig = chainConfig,
        payloads = payloads,
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
 * Maps the deserialized l2-execution proof response DTO onto the domain [L2ExecutionProofResponseV1]
 * described by `rollup_spec/prover_io/schemas/getZkL2ExecutionProofV1.response.schema.json`.
 * The transport is responsible for parsing the JSON (read from a file or returned by a REST call)
 * into [L2ExecutionProofResponseDto] before this mapper runs.
 */
internal object L2ExecutionProofResponseDtoMapper : (
  ExecutionProofIndex,
  L2ExecutionProofResponseDto,
) -> L2ExecutionProofResponseV1 {
  override fun invoke(
    proofIndex: ExecutionProofIndex,
    responseDto: L2ExecutionProofResponseDto,
  ): L2ExecutionProofResponseV1 {
    return L2ExecutionProofResponseV1(
      proverVersion = responseDto.proverVersion,
      startBlockNumber = responseDto.startBlockNumber.toULong(),
      endBlockNumber = responseDto.publicInputs.endBlockNumber.toULong(),
      proof = responseDto.proof.decodeHex(),
      publicInputs = responseDto.publicInputs.toDomainObject(),
      l2L1Messages = responseDto.l2L1Messages.map { it.decodeHex() },
      txFroms = responseDto.txFroms.map { it.decodeHex() },
      filteredAddresses = responseDto.filteredAddresses.map { it.decodeHex() },
    )
  }
}

typealias L2ExecutionProofTransport =
  ProverProofTransport<L2ExecutionProofRequestDto, L2ExecutionProofResponseDto, ExecutionProofIndex>

/**
 * RISC-V execution prover client. The request/response transport is injected
 * via [transport], so the same client works whether requests are written as JSON files or sent over REST.
 */
class L2ExecutionProverClient(
  private val transport: L2ExecutionProofTransport,
  guestProgramId: String,
  chainConfig: ChainConfigDto,
  proofRequestDtoMapper: (L2ExecutionProofRequestV1) -> SafeFuture<L2ExecutionProofRequestDto> =
    L2ExecutionProofRequestDtoMapper(guestProgramId, chainConfig),
  proofResponseDtoMapper: (ExecutionProofIndex, L2ExecutionProofResponseDto) -> L2ExecutionProofResponseV1 =
    L2ExecutionProofResponseDtoMapper,
  log: Logger = LOG,
) : GenericRiscVProverClient<
  L2ExecutionProofRequestV1,
  L2ExecutionProofResponseV1,
  L2ExecutionProofRequestDto,
  L2ExecutionProofResponseDto,
  ExecutionProofIndex,
  >(
  transport = transport,
  proofIndexProvider = { request ->
    ExecutionProofIndex(
      startBlockNumber = request.startBlockNumber,
      endBlockNumber = request.endBlockNumber,
      startBlockTimestamp = request.startBlockTimestamp,
    )
  },
  requestMapper = proofRequestDtoMapper,
  responseMapper = proofResponseDtoMapper,
  proofTypeLabel = "l2-execution",
  log = log,
),
  L2ExecutionProverClientV1 {
  companion object {
    val LOG: Logger = LogManager.getLogger(L2ExecutionProverClient::class.java)
  }
}
