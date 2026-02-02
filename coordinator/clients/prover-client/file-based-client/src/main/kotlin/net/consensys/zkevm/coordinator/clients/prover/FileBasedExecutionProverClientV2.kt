package net.consensys.zkevm.coordinator.clients.prover

import com.fasterxml.jackson.databind.ObjectMapper
import com.fasterxml.jackson.databind.node.ArrayNode
import io.vertx.core.Vertx
import linea.domain.EthLog
import linea.encoding.BlockRLPEncoder
import linea.kotlin.encodeHex
import linea.kotlin.toHexString
import net.consensys.zkevm.coordinator.clients.BatchExecutionProofRequestV1
import net.consensys.zkevm.coordinator.clients.BatchExecutionProofResponse
import net.consensys.zkevm.coordinator.clients.ExecutionProverClientV2
import net.consensys.zkevm.coordinator.clients.prover.serialization.JsonSerialization
import net.consensys.zkevm.domain.ProofIndex
import net.consensys.zkevm.encoding.BlockEncoder
import net.consensys.zkevm.fileio.FileReader
import net.consensys.zkevm.fileio.FileWriter
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.nio.file.Path

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
 * path/to/prover/requests/<startBlockNumber>-<endBlockNumber>--etv<tracesVersion>-stv<stateManagerVersion>-getZkProof.json
 *
 * When done prover will output file
 * path/to/prover/responses/<startBlockNumber>-<endBlockNumber>--etv<tracesVersion>-stv<stateManagerVersion>-getZkProof.json
 *
 * So, this class will need to watch the file system and wait for the output proof to be generated
 */
class FileBasedExecutionProverClientV2(
  config: FileBasedProverConfig,
  private val tracesVersion: String,
  private val stateManagerVersion: String,
  vertx: Vertx,
  jsonObjectMapper: ObjectMapper = JsonSerialization.proofResponseMapperV1,
  executionProofRequestFileNameProvider: ProverFileNameProvider =
    ExecutionProofRequestFileNameProvider(
      tracesVersion = tracesVersion,
      stateManagerVersion = stateManagerVersion,
    ),
  executionProofResponseFileNameProvider: ProverFileNameProvider = ExecutionProofResponseFileNameProvider,
  log: Logger,
) :
  GenericFileBasedProverClient<
    BatchExecutionProofRequestV1,
    BatchExecutionProofResponse,
    BatchExecutionProofRequestDto,
    Any,
    >(
    config = config,
    vertx = vertx,
    fileWriter = FileWriter(vertx, jsonObjectMapper),
    // This won't be used in practice because we don't parse the response
    fileReader = FileReader(vertx, jsonObjectMapper, Any::class.java),
    requestFileNameProvider = executionProofRequestFileNameProvider,
    responseFileNameProvider = executionProofResponseFileNameProvider,
    requestMapper = ExecutionProofRequestDtoMapper(),
    proofIndexProvider = { request ->
      ProofIndex(
        startBlockNumber = request.startBlockNumber,
        endBlockNumber = request.endBlockNumber,
      )
    },
    responseMapper = { throw UnsupportedOperationException("Batch execution proof response shall not be parsed!") },
    proofTypeLabel = "batch",
    log = log,
  ),
  ExecutionProverClientV2 {

  override fun parseResponse(responseFilePath: Path, proofIndex: ProofIndex): SafeFuture<BatchExecutionProofResponse> {
    return SafeFuture.completedFuture(
      BatchExecutionProofResponse(
        startBlockNumber = proofIndex.startBlockNumber,
        endBlockNumber = proofIndex.endBlockNumber,
      ),
    )
  }
  companion object {
    val LOG: Logger = LogManager.getLogger(FileBasedExecutionProverClientV2::class.java)
  }
}
