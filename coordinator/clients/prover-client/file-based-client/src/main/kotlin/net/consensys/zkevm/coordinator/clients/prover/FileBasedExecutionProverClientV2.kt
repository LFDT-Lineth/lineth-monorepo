package net.consensys.zkevm.coordinator.clients.prover

import com.fasterxml.jackson.databind.ObjectMapper
import com.fasterxml.jackson.databind.node.ArrayNode
import io.vertx.core.Vertx
import linea.encoding.BlockRLPEncoder
import net.consensys.encodeHex
import net.consensys.linea.async.toSafeFuture
import net.consensys.toBigInteger
import net.consensys.zkevm.coordinator.clients.BatchExecutionProofRequestV1
import net.consensys.zkevm.coordinator.clients.BatchExecutionProofResponse
import net.consensys.zkevm.coordinator.clients.ExecutionProverClientV2
import net.consensys.zkevm.coordinator.clients.L2MessageServiceLogsClient
import net.consensys.zkevm.coordinator.clients.prover.serialization.JsonSerialization
import net.consensys.zkevm.domain.ProofIndex
import net.consensys.zkevm.domain.RlpBridgeLogsData
import net.consensys.zkevm.encoding.BlockEncoder
import net.consensys.zkevm.fileio.FileReader
import net.consensys.zkevm.fileio.FileWriter
import org.apache.logging.log4j.LogManager
import org.web3j.protocol.Web3j
import org.web3j.protocol.core.DefaultBlockParameter
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.nio.file.Path

data class BatchExecutionProofRequestDto(
  val zkParentStateRootHash: String?,
  val keccakParentStateRootHash: String,
  val conflatedExecutionTracesFile: String,
  val tracesEngineVersion: String,
  val type2StateManagerVersion: String?,
  val zkStateMerkleProof: ArrayNode,
  val blocksData: List<RlpBridgeLogsData>
)

internal class ExecutionProofRequestDataDecorator(
  private val l2MessageServiceLogsClient: L2MessageServiceLogsClient,
  private val l2Web3jClient: Web3j,
  private val encoder: BlockEncoder = BlockRLPEncoder
) : (BatchExecutionProofRequestV1) -> SafeFuture<BatchExecutionProofRequestDto> {
  private fun getBlockStateRootHash(blockNumber: ULong): SafeFuture<String> {
    return l2Web3jClient
      .ethGetBlockByNumber(
        DefaultBlockParameter.valueOf(blockNumber.toBigInteger()),
        false
      )
      .sendAsync()
      .thenApply { block -> block.block.stateRoot }
      .toSafeFuture()
  }

  override fun invoke(request: BatchExecutionProofRequestV1): SafeFuture<BatchExecutionProofRequestDto> {
    val bridgeLogsSfList = request.blocks.map { block ->
      l2MessageServiceLogsClient.getBridgeLogs(blockNumber = block.number.toLong())
        .thenApply { block to it }
    }

    return SafeFuture.collectAll(bridgeLogsSfList.stream())
      .thenCombine(
        getBlockStateRootHash(request.blocks.first().number.toULong() - 1UL)
      ) { blocksAndBridgeLogs, previousKeccakStateRootHash ->
        BatchExecutionProofRequestDto(
          zkParentStateRootHash = request.type2StateData.zkParentStateRootHash.encodeHex(),
          keccakParentStateRootHash = previousKeccakStateRootHash,
          conflatedExecutionTracesFile = request.tracesResponse.tracesFileName,
          tracesEngineVersion = request.tracesResponse.tracesEngineVersion,
          type2StateManagerVersion = request.type2StateData.zkStateManagerVersion,
          zkStateMerkleProof = request.type2StateData.zkStateMerkleProof,
          blocksData = blocksAndBridgeLogs.map { (block, bridgeLogs) ->
            val rlp = encoder.encode(block).encodeHex()
            RlpBridgeLogsData(rlp, bridgeLogs)
          }
        )
      }
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
  l2MessageServiceLogsClient: L2MessageServiceLogsClient,
  vertx: Vertx,
  l2Web3jClient: Web3j,
  jsonObjectMapper: ObjectMapper = JsonSerialization.proofResponseMapperV1,
  executionProofRequestFileNameProvider: ProverFileNameProvider =
    ExecutionProofRequestFileNameProvider(
      tracesVersion = tracesVersion,
      stateManagerVersion = stateManagerVersion
    ),
  executionProofResponseFileNameProvider: ProverFileNameProvider = ExecutionProofResponseFileNameProvider
) :
  GenericFileBasedProverClient<
    BatchExecutionProofRequestV1,
    BatchExecutionProofResponse,
    BatchExecutionProofRequestDto,
    Any
    >(
    config = config,
    vertx = vertx,
    fileWriter = FileWriter(vertx, jsonObjectMapper),
    // This won't be used in practice because we don't parse the response
    fileReader = FileReader(vertx, jsonObjectMapper, Any::class.java),
    requestFileNameProvider = executionProofRequestFileNameProvider,
    responseFileNameProvider = executionProofResponseFileNameProvider,
    requestMapper = ExecutionProofRequestDataDecorator(l2MessageServiceLogsClient, l2Web3jClient),
    responseMapper = { throw UnsupportedOperationException("Batch execution proof response shall not be parsed!") },
    proofTypeLabel = "batch",
    log = LogManager.getLogger(FileBasedExecutionProverClientV2::class.java)
  ),
  ExecutionProverClientV2 {

  override fun parseResponse(
    responseFilePath: Path,
    proofIndex: ProofIndex
  ): SafeFuture<BatchExecutionProofResponse> {
    return SafeFuture.completedFuture(
      BatchExecutionProofResponse(
        startBlockNumber = proofIndex.startBlockNumber,
        endBlockNumber = proofIndex.endBlockNumber
      )
    )
  }
}
