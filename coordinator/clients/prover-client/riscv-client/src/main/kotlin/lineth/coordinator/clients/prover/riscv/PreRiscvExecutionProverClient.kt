package lineth.coordinator.clients.prover.riscv

import com.fasterxml.jackson.databind.ObjectMapper
import com.fasterxml.jackson.databind.node.ArrayNode
import io.vertx.core.Vertx
import linea.clients.BatchExecutionProofRequestV1
import linea.clients.BatchExecutionProofResponse
import linea.clients.ExecutionProverClientV2
import linea.clients.ProverFileNameProvider
import linea.domain.EthLog
import linea.domain.ExecutionProofIndex
import linea.kotlin.encodeHex
import linea.kotlin.toHexString
import lineth.coordinator.clients.prover.serialization.JsonSerialization
import lineth.encoding.BlockEncoder
import lineth.encoding.BlockRLPEncoder
import lineth.fileio.FileReader
import lineth.fileio.FileWriter
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture

data class BatchExecutionProofRequestDto(
  val zkParentStateRootHash: String?,
  val keccakParentStateRootHash: String,
  val conflatedExecutionTracesFile: String,
  val tracesEngineVersion: String,
  val type2StateManagerVersion: String?,
  val zkStateMerkleProof: ArrayNode,
  val blocksData: List<RlpBridgeLogsDto>,
)

data class RlpBridgeLogsDto(val rlp: String, val bridgeLogs: List<BridgeLogsDto>)

data class BridgeLogsDto(
  val removed: Boolean,
  val logIndex: String,
  val transactionIndex: String,
  val transactionHash: String,
  val blockHash: String,
  val blockNumber: String,
  val address: String,
  val data: String,
  val topics: List<String>,
) {
  companion object {
    fun fromDomainObject(ethLog: EthLog): BridgeLogsDto {
      return BridgeLogsDto(
        removed = ethLog.removed,
        logIndex = ethLog.logIndex.toHexString(),
        transactionIndex = ethLog.transactionIndex.toHexString(),
        transactionHash = ethLog.transactionHash.encodeHex(),
        blockHash = ethLog.blockHash.encodeHex(),
        blockNumber = ethLog.blockNumber.toHexString(),
        address = ethLog.address.encodeHex(),
        data = ethLog.data.encodeHex(),
        topics = ethLog.topics.map { it.encodeHex() },
      )
    }
  }
}

internal class ExecutionProofRequestDtoMapper(
  private val encoder: BlockEncoder = BlockRLPEncoder,
) : (BatchExecutionProofRequestV1) -> SafeFuture<BatchExecutionProofRequestDto> {
  override fun invoke(request: BatchExecutionProofRequestV1): SafeFuture<BatchExecutionProofRequestDto> {
    val blocksData = request.blocks.map { block ->
      val rlp = encoder.encode(block).encodeHex()
      val bridgeLogs = request.bridgeLogs.filter {
        it.blockNumber == block.number
      }
      RlpBridgeLogsDto(rlp, bridgeLogs.map(BridgeLogsDto::fromDomainObject))
    }

    return SafeFuture.completedFuture(
      BatchExecutionProofRequestDto(
        zkParentStateRootHash = request.type2StateData.zkParentStateRootHash.encodeHex(),
        keccakParentStateRootHash = request.keccakParentStateRootHash.encodeHex(),
        conflatedExecutionTracesFile = request.tracesResponse.tracesFileName,
        tracesEngineVersion = request.tracesResponse.tracesEngineVersion,
        type2StateManagerVersion = request.type2StateData.zkStateManagerVersion,
        zkStateMerkleProof = request.type2StateData.zkStateMerkleProof,
        blocksData = blocksData,
      ),
    )
  }
}

/**
 * Implementation of interface with the Execution Prover through Files.
 *
 * Prover will ingest file like
 * path/to/prover/requests/<startBlockNumber>-<endBlockNumber>--getZkProof.json
 *
 * When done prover will output file
 * path/to/prover/responses/<startBlockNumber>-<endBlockNumber>-getZkProof.json
 *
 * So, this class will need to watch the file system and wait for the output proof to be generated
 */
class PreRiscvExecutionProverClient(
  config: FileBasedProverConfig,
  vertx: Vertx,
  proofRequestDtoMapper: (BatchExecutionProofRequestV1) -> SafeFuture<BatchExecutionProofRequestDto> =
    ExecutionProofRequestDtoMapper(),
  jsonObjectMapper: ObjectMapper = JsonSerialization.proofResponseMapperV1,
  executionProofRequestFileNameProvider: ProverFileNameProvider<ExecutionProofIndex> =
    ExecutionProofFileNameProvider,
  executionProofResponseFileNameProvider: ProverFileNameProvider<ExecutionProofIndex> =
    ExecutionProofFileNameProvider,
  log: Logger = LOG,
) :
  GenericRiscVProverClient<
    BatchExecutionProofRequestV1,
    BatchExecutionProofResponse,
    BatchExecutionProofRequestDto,
    Any,
    ExecutionProofIndex,
    >(
    transport = FileBasedProverProofTransport(
      config = config,
      vertx = vertx,
      fileWriter = FileWriter(vertx, jsonObjectMapper),
      fileReader = FileReader(vertx, jsonObjectMapper, Any::class.java),
      requestFileNameProvider = executionProofRequestFileNameProvider,
      responseFileNameProvider = executionProofResponseFileNameProvider,
    ),
    proofIndexProvider = { request ->
      ExecutionProofIndex(
        startBlockNumber = request.startBlockNumber,
        endBlockNumber = request.endBlockNumber,
        startBlockTimestamp = request.startBlockTimestamp,
      )
    },
    requestMapper = proofRequestDtoMapper,
    responseMapper = {
      throw UnsupportedOperationException("Batch execution proof response shall not be parsed!")
    },
    proofTypeLabel = "batch",
    log = log,
  ),
  ExecutionProverClientV2 {

  companion object {
    val LOG: Logger = LogManager.getLogger(PreRiscvExecutionProverClient::class.java)
  }
}
