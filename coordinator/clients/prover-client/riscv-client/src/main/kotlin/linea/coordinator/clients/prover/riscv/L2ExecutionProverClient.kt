package linea.coordinator.clients.prover.riscv

import linea.clients.L2ExecutionProofRequestV1
import linea.clients.L2ExecutionProofResponse
import linea.clients.L2ExecutionProverClientV1
import linea.domain.ExecutionProofIndex
import linea.encoding.BlockEncoder
import linea.encoding.BlockRLPEncoder
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
  private val proverVersion: String,
  private val chainConfig: ChainConfigDto,
  private val encoder: BlockEncoder = BlockRLPEncoder,
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
    val blocks = request.blocks.map { block ->
      L2BlockDto(
        blockRlp = encoder.encode(block).encodeHex(),
        // TODO: forced-transaction metadata (§6.5) is not present on L2ExecutionProofRequestV1.
        forcedTransactions = request.forcedTransactions.filter { it -> it.blockNumber == block.number }.map {
          ForcedTransactionDto(
            ftxNumber = it.ftxNumber.toLong(),
            deadlineBlockNumber = it.deadlineBlockNumber.toLong(),
            signedTxRlp = it.signedTxRlp.encodeHex(),
            acceptance = mapFtxInclusionResultToAcceptance(it.acceptance),
          )
        },
      )
    }

    val dto = L2ExecutionProofRequestDto(
      proverVersion = proverVersion,
      blockRange = BlockRangeDto(
        startBlockNumber = request.startBlockNumber.toLong(),
        endBlockNumber = request.endBlockNumber.toLong(),
      ),
      // TODO: the 15-field public-inputs tuple is not derivable from L2ExecutionProofRequestV1 yet.
      publicInputs = L2ExecutionProofPublicInputsDto(
        parentBlockHash = request.blocks.first().parentHash.encodeHex(),
        endBlockHash = request.blocks.last().hash.encodeHex(),
        endBlockNumber = request.endBlockNumber.toLong(),
        endBlockTimestamp = request.blocks.last().timestamp.toLong(),
        L2L1MessagesHash = request.l2L1MessagesHash.encodeHex(),
        parentL1L2BridgeRollingHash = request.parentL1L2BridgeRollingHash.encodeHex(),
        parentL1L2BridgeRollingHashMessageNumber = request.parentL1L2BridgeRollingHashMessageNumber.toLong(),
        endL1L2BridgeRollingHash = request.endL1L2BridgeRollingHash.encodeHex(),
        endL1L2BridgeRollingHashMessageNumber = request.endL1L2BridgeRollingHashMessageNumber.toLong(),
        dynamicChainConfigHash = request.dynamicChainConfigHash.encodeHex(),
        parentFtxRollingHash = request.parentFtxRollingHash.encodeHex(),
        endFtxRollingHash = request.endFtxRollingHash.encodeHex(),
        lastProcessedFtxNumber = request.lastProcessedFtxNumber.toLong(),
        filteredAddressesHash = request.filteredAddressesHash.encodeHex(),
        txFromsHash = request.txFromsHash.encodeHex(),
      ),
      chainConfig = chainConfig,
      blocks = blocks,
      // TODO: per-block execution witness (debug_executionWitness output) is not carried by the domain request yet.
      executionWitness = emptyList(),
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
      startBlockNumber = proofIndex.startBlockNumber,
      endBlockNumber = proofIndex.endBlockNumber,
      proof = responseDto.proof.decodeHex(),
      publicInputs = responseDto.publicInputs.toDomainObject(),
      L2L1MsgList = responseDto.L2L1MsgList.map { it.decodeHex() },
      froms = responseDto.froms.map { it.decodeHex() },
      addrs = responseDto.addrs.map { it.decodeHex() },
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
  proverVersion: String,
  chainConfig: ChainConfigDto,
  proofRequestDtoMapper: (L2ExecutionProofRequestV1) -> SafeFuture<L2ExecutionProofRequestDto> =
    L2ExecutionProofRequestDtoMapper(proverVersion, chainConfig),
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
