package linea.ftx.conflation

import build.linea.clients.GetZkEVMStateMerkleProofResponse
import build.linea.clients.LineaAccountProof
import build.linea.clients.StateManagerAccountProofClient
import build.linea.clients.StateManagerClientV1
import com.github.michaelbull.result.getOrThrow
import linea.clients.GenerateTracesResponse
import linea.clients.InvalidityProofRequest
import linea.clients.InvalidityProofResponse
import linea.clients.InvalidityProverClientV1
import linea.clients.InvalidityReason
import linea.clients.TracesConflationVirtualBlockClientV1
import linea.contract.events.ForcedTransactionAddedEvent
import linea.domain.BlockInterval
import linea.domain.BlockParameter
import linea.domain.InvalidityProofIndex
import linea.ethapi.EthLogsClient
import linea.ethapi.EthLogsFilterOptions
import linea.forcedtx.ForcedTransactionInclusionResult
import linea.kotlin.toHexStringUInt256
import linea.persistence.ForcedTransactionRecord
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.apache.tuweni.bytes.Bytes
import org.hyperledger.besu.ethereum.core.encoding.EncodingContext
import org.hyperledger.besu.ethereum.core.encoding.TransactionDecoder
import tech.pegasys.teku.infrastructure.async.SafeFuture

/**
 * Responsible for fetching all the necessary data from Shomei and Tracer clients to
 * assemble forced transactions invalidity proofs request and send it to the prover client.
 */
class InvalidityProofAssembler(
  private val invalidityProofClient: InvalidityProverClientV1,
  private val stateManagerClient: StateManagerClientV1,
  private val accountProofClient: StateManagerAccountProofClient,
  private val ethApiLogsClient: EthLogsClient,
  private val tracesClient: TracesConflationVirtualBlockClientV1,
  private val contractAddress: String,
  private val log: Logger = LogManager.getLogger(InvalidityProofAssembler::class.java),
) {

  /**
   * Requests an invalidity proof for a failed forced transaction.
   *
   * @param ftx The forced transaction record containing execution status
   * @return SafeFuture<InvalidityProofResponse> containing the proof request result
   */
  fun requestInvalidityProof(
    ftx: ForcedTransactionRecord,
  ): SafeFuture<InvalidityProofResponse> {
    log.info(
      "assembling forced transaction invalidity proof: ftx={} with inclusionResult={} at block={}",
      ftx.ftxNumber,
      ftx.inclusionResult,
      ftx.simulatedExecutionBlockNumber,
    )

    return invalidityProofClient
      .isProofAlreadyDone(
        InvalidityProofIndex(
          simulatedExecutionBlockNumber = ftx.simulatedExecutionBlockNumber,
          ftxNumber = ftx.ftxNumber,
          startBlockTimestamp = ftx.simulatedExecutionBlockTimestamp,
        ),
      )
      .thenCompose { alreadyDone ->
        if (alreadyDone) {
          SafeFuture.completedFuture(InvalidityProofResponse(ftx.ftxNumber))
        } else {
          val invalidityReason = mapInclusionResultToInvalidityReason(ftx.inclusionResult)
          fetchRequiredDataForInvalidityProof(invalidityReason, ftx)
            .thenCompose { requiredData ->
              val request = buildInvalidityProofRequest(
                invalidityReason = invalidityReason,
                ftxRecord = ftx,
                requiredData = requiredData,
              )
              invalidityProofClient.requestProof(request)
            }
        }
      }
  }

  private fun getPrevFtxRollingHash(ftxNumber: ULong): SafeFuture<ByteArray> {
    if (ftxNumber == 1uL) {
      return SafeFuture.completedFuture(ByteArray(32))
    }
    val prevFtxNumber = ftxNumber - 1uL
    return ethApiLogsClient
      .ethGetLogs(
        filterOptions = EthLogsFilterOptions(
          fromBlock = BlockParameter.Tag.EARLIEST,
          toBlock = BlockParameter.Tag.LATEST,
          address = contractAddress,
          topics = listOf(
            ForcedTransactionAddedEvent.topic,
            prevFtxNumber.toHexStringUInt256(),
          ),
        ),
      ).thenApply { logs ->
        if (logs.isEmpty()) {
          throw IllegalStateException("No ForcedTransactionAdded event found for ftx=$prevFtxNumber")
        }
        ForcedTransactionAddedEvent.fromEthLog(logs.first()).event.forcedTransactionRollingHash
      }
  }

  private fun mapInclusionResultToInvalidityReason(
    inclusionResult: ForcedTransactionInclusionResult,
  ): InvalidityReason {
    return when (inclusionResult) {
      ForcedTransactionInclusionResult.Included -> InvalidityReason.BadNonce
      ForcedTransactionInclusionResult.BadNonce -> InvalidityReason.BadNonce
      ForcedTransactionInclusionResult.BadBalance -> InvalidityReason.BadBalance
      ForcedTransactionInclusionResult.BadPrecompile -> InvalidityReason.BadPrecompile
      ForcedTransactionInclusionResult.TooManyLogs -> InvalidityReason.TooManyLogs
      ForcedTransactionInclusionResult.FilteredAddressFrom -> InvalidityReason.FilteredAddressFrom
      ForcedTransactionInclusionResult.FilteredAddressTo -> InvalidityReason.FilteredAddressTo
      ForcedTransactionInclusionResult.Phylax ->
        throw IllegalArgumentException("Phylax invalidity proofs are not supported yet")
    }
  }

  private data class RequiredInvalidityProofData(
    val prevFtxRollingHash: ByteArray,
    val zkParentStateRootHash: ByteArray,
    val tracesFile: String? = null,
    val accountProof: LineaAccountProof? = null,
    val zkStateMerkleProof: GetZkEVMStateMerkleProofResponse? = null,
  ) {
    override fun equals(other: Any?): Boolean {
      if (this === other) return true
      if (javaClass != other?.javaClass) return false

      other as RequiredInvalidityProofData

      if (!prevFtxRollingHash.contentEquals(other.prevFtxRollingHash)) return false
      if (!zkParentStateRootHash.contentEquals(other.zkParentStateRootHash)) return false
      if (tracesFile != other.tracesFile) return false
      if (accountProof != other.accountProof) return false
      if (zkStateMerkleProof != other.zkStateMerkleProof) return false

      return true
    }

    override fun hashCode(): Int {
      var result = prevFtxRollingHash.contentHashCode()
      result = 31 * result + zkParentStateRootHash.contentHashCode()
      result = 31 * result + (tracesFile?.hashCode() ?: 0)
      result = 31 * result + (accountProof?.hashCode() ?: 0)
      result = 31 * result + (zkStateMerkleProof?.hashCode() ?: 0)
      return result
    }
  }

  private fun fetchRequiredDataForInvalidityProof(
    invalidityReason: InvalidityReason,
    ftx: ForcedTransactionRecord,
  ): SafeFuture<RequiredInvalidityProofData> {
    val prevFtxRollingHashFuture = getPrevFtxRollingHash(ftx.ftxNumber)
    val zkParentStateRootHashFuture = stateManagerClient
      .rollupGetStateMerkleProof(BlockInterval(ftx.simulatedExecutionBlockNumber, ftx.simulatedExecutionBlockNumber))
    var accountProofFuture: SafeFuture<LineaAccountProof?> = SafeFuture.completedFuture(null)
    var stateProofFuture: SafeFuture<GetZkEVMStateMerkleProofResponse?> = SafeFuture.completedFuture(null)
    var tracesFuture: SafeFuture<GenerateTracesResponse?> = SafeFuture.completedFuture(null)
    val from =
      TransactionDecoder.decodeOpaqueBytes(
        Bytes.wrap(ftx.ftxRlp),
        EncodingContext.POOLED_TRANSACTION,
      ).sender.bytes.toArray()
    if (invalidityReason == InvalidityReason.BadNonce || invalidityReason == InvalidityReason.BadBalance) {
      accountProofFuture = fetchAccountProof(from, ftx.simulatedExecutionBlockNumber)
        .thenApply { it }
    }
    if (invalidityReason == InvalidityReason.BadPrecompile || invalidityReason == InvalidityReason.TooManyLogs) {
      stateProofFuture = stateManagerClient.rollupGetVirtualStateMerkleProof(
        ftx.simulatedExecutionBlockNumber,
        ftx.ftxRlp,
      ).thenApply { it }
      tracesFuture = tracesClient.generateVirtualBlockConflatedTracesToFile(
        ftx.simulatedExecutionBlockNumber,
        ftx.ftxRlp,
      )
        .thenApply { it.getOrThrow { errorResponse -> errorResponse.asException() } }
    }
    return SafeFuture
      .allOf(
        prevFtxRollingHashFuture,
        zkParentStateRootHashFuture,
        accountProofFuture,
        stateProofFuture,
        tracesFuture,
      )
      .thenApply {
        RequiredInvalidityProofData(
          prevFtxRollingHash = prevFtxRollingHashFuture.get(),
          zkParentStateRootHash = zkParentStateRootHashFuture.get().zkParentStateRootHash,
          tracesFile = tracesFuture.get()?.tracesFileName,
          accountProof = accountProofFuture.get(),
          zkStateMerkleProof = stateProofFuture.get(),
        )
      }
  }

  private fun buildInvalidityProofRequest(
    invalidityReason: InvalidityReason,
    ftxRecord: ForcedTransactionRecord,
    requiredData: RequiredInvalidityProofData,
  ): InvalidityProofRequest {
    return InvalidityProofRequest(
      invalidityReason = invalidityReason,
      simulatedExecutionBlockNumber = ftxRecord.simulatedExecutionBlockNumber,
      simulatedExecutionBlockTimestamp = ftxRecord.simulatedExecutionBlockTimestamp,
      ftxNumber = ftxRecord.ftxNumber,
      ftxBlockNumberDeadline = ftxRecord.ftxBlockNumberDeadline,
      ftxRlp = ftxRecord.ftxRlp,
      prevFtxRollingHash = requiredData.prevFtxRollingHash,
      zkParentStateRootHash = requiredData.zkParentStateRootHash,
      tracesResponse = requiredData.tracesFile,
      accountProof = requiredData.accountProof,
      zkStateMerkleProof = requiredData.zkStateMerkleProof,
    )
  }

  private fun fetchAccountProof(
    address: ByteArray,
    blockNumber: ULong,
  ): SafeFuture<LineaAccountProof> {
    return accountProofClient.lineaGetAccountProof(
      address = address,
      storageKeys = emptyList(), // No storage keys needed for nonce/balance proofs
      blockNumber = blockNumber,
    )
  }
}
