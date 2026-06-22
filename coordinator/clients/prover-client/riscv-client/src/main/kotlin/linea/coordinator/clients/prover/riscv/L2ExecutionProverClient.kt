package linea.coordinator.clients.prover.riscv

import linea.clients.L2ExecutionProofRequestV1
import linea.clients.L2ExecutionProofResponse
import linea.clients.L2ExecutionProverClientV1
import linea.domain.ExecutionProofIndex
import linea.forcedtx.ForcedTransactionInclusionResult
import linea.kotlin.decodeHex
import linea.kotlin.encodeHex
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture

/**
 * Maps a [L2ExecutionProofRequestV1] domain request to the RISC-V l2-execution proof request DTO described by
 * `rollup_spec/prover_inputs/getZkL2ExecutionProof.request.json` (underscore-prefixed documentation fields skipped).
 *
 * NOTE: the legacy [L2ExecutionProofRequestV1] does not yet carry several inputs the RISC-V schema requires
 * (the 15-field public-inputs tuple, the per-block execution witness, and per-block forced-transaction metadata).
 * Those fields are flagged with `TODO` until the upstream request plumbing provides them.
 */
internal class L2ExecutionProofRequestDtoMapper(
  private val guestProgramId: String,
  private val chainConfig: ChainConfigDto,
) : (L2ExecutionProofRequestV1) -> SafeFuture<L2ExecutionProofRequestDto> {
  private fun mapFtxInclusionResultToAcceptance(
    inclusionResult: ForcedTransactionInclusionResult,
  ): ForcedTransactionAcceptance {
    return when (inclusionResult) {
      ForcedTransactionInclusionResult.Included -> ForcedTransactionAcceptance.INCLUDED
      ForcedTransactionInclusionResult.BadNonce -> ForcedTransactionAcceptance.BAD_NONCE
      ForcedTransactionInclusionResult.BadBalance -> ForcedTransactionAcceptance.BAD_BALANCE
      ForcedTransactionInclusionResult.FilteredAddressFrom -> ForcedTransactionAcceptance.FILTERED_ADDRESS_FROM
      ForcedTransactionInclusionResult.FilteredAddressTo -> ForcedTransactionAcceptance.FILTERED_ADDRESS_TO
      else -> throw IllegalArgumentException("Unsupported FTX inclusion result: $inclusionResult")
    }
  }

  override fun invoke(request: L2ExecutionProofRequestV1): SafeFuture<L2ExecutionProofRequestDto> {
    val payloads = request.executionPayloads.map { executionPayload ->
      val blockNumber = executionPayload.blockNumber
      val executionWitness = request.executionWitnesses.find { it.blockNumber == blockNumber }
      val executionRequests = request.executionRequests.find { it.blockNumber == blockNumber }
      val forcedTransactions = request.forcedTransactions.filter { it -> it.blockNumber == blockNumber }
      val statelessInputDto = StatelessInputDto(
        newPayloadRequest = NewPayloadRequestDto(
          executionPayload = ExecutionPayloadDto(
            parentHash = executionPayload.parentHash.encodeHex(),
            feeRecipient = executionPayload.feeRecipient.encodeHex(),
            stateRoot = executionPayload.stateRoot.encodeHex(),
            receiptsRoot = executionPayload.receiptsRoot.encodeHex(),
            logsBloom = executionPayload.logsBloom.encodeHex(),
            prevRandao = executionPayload.prevRandao.encodeHex(),
            blockNumber = executionPayload.blockNumber.toLong(),
            gasLimit = executionPayload.gasLimit.toLong(),
            gasUsed = executionPayload.gasUsed.toLong(),
            timestamp = executionPayload.timestamp.toLong(),
            extraData = executionPayload.extraData.encodeHex(),
            baseFeePerGas = executionPayload.baseFeePerGas,
            blockHash = executionPayload.blockHash.encodeHex(),
            transactions = executionPayload.transactions.map { it.encodeHex() },
            withdrawals = executionPayload.withdrawals.map {
              WithdrawalDto(
                index = it.index.toLong(),
                validatorIndex = it.validatorIndex.toLong(),
                address = it.address.encodeHex(),
                amount = it.amount.toLong(),
              )
            },
            blobGasUsed = executionPayload.blobGasUsed.toLong(),
            excessBlobGas = executionPayload.excessBlobGas.toLong(),
            blockAccessList = executionPayload.blockAccessList.encodeHex(),
          ),
          versionedHashes = emptyList(),
          parentBeaconBlockRoot = ByteArray(32).encodeHex(),
          executionRequests = ExecutionRequestsDto(
            deposits = executionRequests?.deposits?.map { it.encodeHex() } ?: emptyList(),
            withdrawals = executionRequests?.withdrawals?.map { it.encodeHex() } ?: emptyList(),
            consolidations = executionRequests?.consolidations?.map { it.encodeHex() } ?: emptyList(),
          ),
        ),
        executionWitness = executionWitness!!.let { execWithness ->
          ExecutionWitnessDto(
            state = execWithness.state.map { it.encodeHex() },
            keys = execWithness.keys.map { it.encodeHex() },
            codes = execWithness.codes.map { it.encodeHex() },
            headers = execWithness.headers.map { it.encodeHex() },
          )
        },
        chainConfig = StatelessChainConfigDto(
          chainId = request.chainConfig.chainId.toLong(),
          forkName = "Amsterdam",
        ),
        publicKeys = emptyList(),
      )
      PayloadInputDto(
        statelessInput = statelessInputDto,
        rollupExtension = RollupExtensionDto(
          forcedTransactions = forcedTransactions.map {
            ForcedTransactionDto(
              number = it.ftxNumber.toLong(),
              deadline = it.deadlineBlockNumber.toLong(),
              signedTxRlp = it.signedTxRlp.encodeHex(),
              acceptance = mapFtxInclusionResultToAcceptance(it.acceptance),
            )
          },
        ),
      )
    }

    val dto = L2ExecutionProofRequestDto(
      guestProgramId = guestProgramId,
      proofRequest = L2ExecutionProofRequestParamsDto(
        parentFtxRollingHash = request.parentFtxRollingHash.encodeHex(),
        parentLastProcessedFtxNumber = request.parentLastProcessedFtxNumber.toLong(),
        chainConfig = chainConfig,
        payloads = payloads,
      ),
    )

    return SafeFuture.completedFuture(dto)
  }
}

/**
 * Maps the deserialized l2-execution proof response DTO onto the domain [L2ExecutionProofResponse]. Field names and
 * types are identical between the two, so this is a straight field copy. The transport is responsible for parsing the
 * JSON (read from a file or returned by a REST call) into [L2ExecutionProofResponseDto] before this mapper runs.
 */
internal object L2ExecutionProofResponseDtoMapper : (
  ExecutionProofIndex,
  L2ExecutionProofResponseDto,
) -> L2ExecutionProofResponse {
  override fun invoke(
    proofIndex: ExecutionProofIndex,
    responseDto: L2ExecutionProofResponseDto,
  ): L2ExecutionProofResponse {
    return L2ExecutionProofResponse(
      startBlockNumber = responseDto.startBlockNumber.toULong(),
      endBlockNumber = responseDto.endBlockNumber.toULong(),
      proof = responseDto.proof.decodeHex(),
      publicInputs = responseDto.publicInputs.toDomainObject(),
      l2L1Messages = responseDto.l2L1Messages.map { it.decodeHex() },
      txFroms = responseDto.txFroms.map { it.decodeHex() },
      filteredAddresses = responseDto.filteredAddresses.map { it.decodeHex() },
    )
  }
}

private typealias L2ExecutionProofTransport =
  ProverProofTransport<L2ExecutionProofRequestDto, L2ExecutionProofResponseDto, ExecutionProofIndex>

/**
 * RISC-V execution prover client. Unlike the file-based execution client, the request/response transport is injected
 * via [transport], so the same client works whether requests are written as JSON files or sent over REST.
 */
class L2ExecutionProverClient(
  private val transport: L2ExecutionProofTransport,
  guestProgramId: String,
  chainConfig: ChainConfigDto,
  proofRequestDtoMapper: (L2ExecutionProofRequestV1) -> SafeFuture<L2ExecutionProofRequestDto> =
    L2ExecutionProofRequestDtoMapper(guestProgramId, chainConfig),
  proofResponseDtoMapper: (ExecutionProofIndex, L2ExecutionProofResponseDto) -> L2ExecutionProofResponse =
    L2ExecutionProofResponseDtoMapper,
  log: Logger = LOG,
) : GenericRiscVProverClient<
  L2ExecutionProofRequestV1,
  L2ExecutionProofResponse,
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
