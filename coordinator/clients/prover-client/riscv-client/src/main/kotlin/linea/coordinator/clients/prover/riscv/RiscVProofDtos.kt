package linea.coordinator.clients.prover.riscv

import java.math.BigInteger

/**
 * Request/response DTOs for the RISC-V provers.
 *
 * These mirror the JSON request fixtures under `rollup_spec/prover_inputs/` one-to-one, EXCLUDING the documentation
 * helper fields whose names start with an underscore (`_comment`, `_comment_*`). Field names are kept identical to
 * the JSON keys so they serialize without custom naming.
 *
 * The DTO <-> domain mapping lives in `RiscVProofDtoMappers.kt`.
 */

/** The 15-field PI tuple emitted by a l2-execution proof (rollup_spec §2.1). */
data class L2ExecutionProofPublicInputsDto(
  val parentBlockHash: String,
  val endBlockHash: String,
  val endBlockNumber: Long,
  val endBlockTimestamp: Long,
  val l2L1MessagesHash: String,
  val parentL1L2BridgeRollingHash: String,
  val parentL1L2BridgeRollingHashMessageNumber: Long,
  val endL1L2BridgeRollingHash: String,
  val endL1L2BridgeRollingHashMessageNumber: Long,
  val dynamicChainConfigHash: String,
  val parentFtxRollingHash: String,
  val endFtxRollingHash: String,
  val lastProcessedFtxNumber: Long,
  val filteredAddressesHash: String,
  val txFromsHash: String,
)

/** The 14-field PI tuple emitted by a rollup / rollup-aggregation proof (rollup_spec §2.4). */
data class RollupProofPublicInputsDto(
  val endBlockNumber: Long,
  val endBlockTimestamp: Long,
  val l2L1BridgeTransactionTree: String,
  val parentL1L2BridgeRollingHash: String,
  val parentL1L2BridgeRollingHashMessageNumber: Long,
  val endL1L2BridgeRollingHash: String,
  val endL1L2BridgeRollingHashMessageNumber: Long,
  val dynamicChainConfigHash: String,
  val parentFtxRollingHash: String,
  val endFtxRollingHash: String,
  val lastProcessedFtxNumber: Long,
  val filteredAddressesHash: String,
  val parentShnarf: String,
  val endShnarf: String,
)

data class MetaDataDto(
  val startBlockNumber: Long,
  val endBlockNumber: Long,
)

// ---------------------------------------------------------------------------------------------------------------------
// getZkL2ExecutionProof.request.json (§2.1)
// ---------------------------------------------------------------------------------------------------------------------

/** One of the five `ForcedTransactionAcceptance` variants (rollup_spec §6.5). */
enum class ForcedTransactionAcceptance {
  INCLUDED,
  BAD_NONCE,
  BAD_BALANCE,
  FILTERED_ADDRESS_FROM,
  FILTERED_ADDRESS_TO,
}

/** Per-FTX metadata (rollup_spec §6.5). */
data class ForcedTransactionDto(
  val number: Long,
  val deadline: Long,
  val signedTxRlp: String,
  val acceptance: ForcedTransactionAcceptance,
)

data class WithdrawalDto(
  val index: Long,
  val validatorIndex: Long,
  val address: String,
  val amount: Long,
)

// ExecutionPayLoadV4 (ExecutionPayloadV3 plus blockAccessList)
data class ExecutionPayloadDto(
  val parentHash: String,
  val feeRecipient: String,
  val stateRoot: String,
  val receiptsRoot: String,
  val logsBloom: String,
  val prevRandao: String,
  val blockNumber: Long,
  val gasLimit: Long,
  val gasUsed: Long,
  val timestamp: Long,
  val extraData: String,
  val baseFeePerGas: BigInteger,
  val blockHash: String,
  val transactions: List<String>,
  val withdrawals: List<WithdrawalDto>,
  val blobGasUsed: Long,
  val excessBlobGas: Long,
  val blockAccessList: String,
)

data class NewPayloadRequestDto(
  val executionPayload: ExecutionPayloadDto,
  val versionedHashes: List<String>,
  val parentBeaconBlockRoot: String,
  val executionRequests: List<String>,
)

/** Static chain configuration (preimage inputs of `dynamicChainConfigHash`). */
data class ChainConfigDto(
  val l2MessageServiceAddress: String,
  val coinbase: String,
  val chainId: Long,
  val forkName: String,
)

/** A single `debug_executionWitness` entry, one per block in canonical order. */
data class ExecutionWitnessDto(
  val state: List<String>,
  val codes: List<String>,
  val headers: List<String>,
)

data class RollupExtensionDto(
  val forcedTransactions: List<ForcedTransactionDto>,
)

data class StatelessInputDto(
  val newPayloadRequest: NewPayloadRequestDto,
  val executionWitness: ExecutionWitnessDto,
)

data class PayloadInputDto(
  val statelessInput: StatelessInputDto,
  val rollupExtension: RollupExtensionDto,
)

data class L2ExecutionProofRequestParamsDto(
  val parentFtxRollingHash: String,
  val parentLastProcessedFtxNumber: Long,
  val payloads: List<PayloadInputDto>,
  val chainConfig: ChainConfigDto,
)

data class L2ExecutionProofRequestDto(
  val guestProgramId: String,
  val proofRequest: L2ExecutionProofRequestParamsDto,
  val metadata: MetaDataDto,
)

data class L2ExecutionProofResponseDto(
  val proverVersion: String,
  val startBlockNumber: Long,
  val proof: String,
  val publicInputs: L2ExecutionProofPublicInputsDto,
  val l2L1Messages: List<String>,
  val txFroms: List<String>,
  val filteredAddresses: List<String>,
)

// ---------------------------------------------------------------------------------------------------------------------
// getZkRollupProof.request.json (§2.2)
// ---------------------------------------------------------------------------------------------------------------------

data class BlobDto(
  val startBlockNumber: Long,
  val endBlockNumber: Long,
  val blobHash: String,
  val blobKzgProof: String,
  val blockRlps: List<String>,
)

/** An inlined l2-execution proof consumed by the rollup guest. */
data class L2ExecutionProofDto(
  val proof: String,
  val startBlockNumber: Long,
  val endBlockNumber: Long,
  val publicInputs: L2ExecutionProofPublicInputsDto,
  val l2L1Messages: List<String>,
  val txFroms: List<String>,
  val filteredAddresses: List<String>,
)

data class RollupProofRequestParamsDto(
  val chainId: Long,
  val blobs: List<BlobDto>,
  val parentShnarf: String,
  val l2ExecutionProofs: List<L2ExecutionProofDto>,
)

data class RollupProofRequestDto(
  val guestProgramId: String,
  val proofRequest: RollupProofRequestParamsDto,
  val metadata: MetaDataDto,
)

data class RollupProofResponseDto(
  val proverVersion: String,
  val startBlockNumber: Long,
  val proof: String,
  val publicInputs: RollupProofPublicInputsDto,
  val l2L1Roots: List<String>,
  val filteredAddresses: List<String>,
)

// ---------------------------------------------------------------------------------------------------------------------
// getZkRollupAggregationProof.request.json (§2.3)
// ---------------------------------------------------------------------------------------------------------------------

/** An inlined rollup proof consumed by the rollup-aggregation guest. */
data class RollupProofDto(
  val proof: String,
  val startBlockNumber: Long,
  val endBlockNumber: Long,
  val publicInputs: RollupProofPublicInputsDto,
  val l2L1Roots: List<String>,
  val filteredAddresses: List<String>,
)

typealias RollupAggregationPublicInputsDto = RollupProofPublicInputsDto

data class RollupAggregationProofRequestParamsDto(
  val rollupProofs: List<RollupProofDto>,
)

data class RollupAggregationProofRequestDto(
  val guestProgramId: String,
  val proofRequest: RollupAggregationProofRequestParamsDto,
  val metadata: MetaDataDto,
)

/** Response of a rollup-aggregation proof: the aggregated proof bytes plus the 14-field PI tuple (§2.4). */
data class RollupAggregationProofResponseDto(
  val proverVersion: String,
  val startBlockNumber: Long,
  val proof: String,
  val publicInputs: RollupAggregationPublicInputsDto,
  val l2L1Roots: List<String>,
  val filteredAddresses: List<String>,
  val l2MessagingBlocksOffsets: List<Long>,
)
