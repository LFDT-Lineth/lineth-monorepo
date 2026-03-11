package net.consensys.zkevm.coordinator.clients.prover

import com.fasterxml.jackson.databind.ObjectMapper
import io.vertx.core.Vertx
import linea.kotlin.encodeHex
import net.consensys.zkevm.coordinator.clients.ProofAggregationProverClientV2
import net.consensys.zkevm.coordinator.clients.prover.serialization.JsonSerialization
import net.consensys.zkevm.coordinator.clients.prover.serialization.ProofToFinalizeJsonResponse
import net.consensys.zkevm.domain.AggregationProofIndex
import net.consensys.zkevm.domain.CompressionProofIndex
import net.consensys.zkevm.domain.ExecutionProofIndex
import net.consensys.zkevm.domain.InvalidityProofIndex
import net.consensys.zkevm.domain.ProofToFinalize
import net.consensys.zkevm.domain.ProofsToAggregate
import net.consensys.zkevm.ethereum.crypto.HashFunction
import net.consensys.zkevm.ethereum.crypto.Sha256HashFunction
import net.consensys.zkevm.fileio.FileReader
import net.consensys.zkevm.fileio.FileWriter
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture

data class AggregationProofRequestDto(
  val executionProofs: List<String>,
  val compressionProofs: List<String>,
  val invalidityProofs: List<String>,
  val parentAggregationLastBlockTimestamp: Long,
  val parentAggregationLastL1RollingHashMessageNumber: Long,
  val parentAggregationLastL1RollingHash: String,
  val parentAggregationLastFtxNumber: Long,
  val parentAggregationLastFtxRollingHash: String,
) {
  companion object {
    fun fromDomainObject(
      proofsToAggregate: ProofsToAggregate,
      executionProofResponseFileNameProvider: ProverFileNameProvider<ExecutionProofIndex>,
      compressionProofResponseFileNameProvider: ProverFileNameProvider<CompressionProofIndex>,
      invalidityProofResponseFileNameProvider: ProverFileNameProvider<InvalidityProofIndex>,
    ): AggregationProofRequestDto {
      val executionProofsResponseFiles = proofsToAggregate.executionProofs
        .toIntervalList()
        .map { blockInterval ->
          executionProofResponseFileNameProvider.getFileName(
            ExecutionProofIndex(
              startBlockNumber = blockInterval.startBlockNumber,
              endBlockNumber = blockInterval.endBlockNumber,
            ),
          )
        }

      val compressionProofsResponsesFiles = proofsToAggregate.compressionProofIndexes
        .map {
          compressionProofResponseFileNameProvider.getFileName(
            CompressionProofIndex(
              startBlockNumber = it.startBlockNumber,
              endBlockNumber = it.endBlockNumber,
              hash = it.hash,
            ),
          )
        }

      val invalidityProofsResponsesFiles =
        proofsToAggregate.invalidityProofs.map(invalidityProofResponseFileNameProvider::getFileName)

      return AggregationProofRequestDto(
        executionProofs = executionProofsResponseFiles,
        compressionProofs = compressionProofsResponsesFiles,
        invalidityProofs = invalidityProofsResponsesFiles,
        parentAggregationLastBlockTimestamp = proofsToAggregate.parentAggregationLastBlockTimestamp.epochSeconds,
        parentAggregationLastL1RollingHashMessageNumber =
        proofsToAggregate.parentAggregationLastL1RollingHashMessageNumber.toLong(),
        parentAggregationLastL1RollingHash = proofsToAggregate.parentAggregationLastL1RollingHash.encodeHex(),
        parentAggregationLastFtxNumber = proofsToAggregate.parentAggregationLastFtxNumber.toLong(),
        parentAggregationLastFtxRollingHash = proofsToAggregate.parentAggregationLastFtxRollingHash.encodeHex(),
      )
    }
  }
}

internal class AggregationRequestDtoMapper(
  private val executionProofResponseFileNameProvider: ProverFileNameProvider<ExecutionProofIndex>,
  private val compressionProofResponseFileNameProvider: ProverFileNameProvider<CompressionProofIndex>,
  private val invalidityProofResponseFileNameProvider: ProverFileNameProvider<InvalidityProofIndex>,
) : (ProofsToAggregate) -> SafeFuture<AggregationProofRequestDto> {
  override fun invoke(proofsToAggregate: ProofsToAggregate): SafeFuture<AggregationProofRequestDto> {
    return SafeFuture.completedFuture(
      AggregationProofRequestDto.fromDomainObject(
        proofsToAggregate,
        executionProofResponseFileNameProvider,
        compressionProofResponseFileNameProvider,
        invalidityProofResponseFileNameProvider,
      ),
    )
  }
}

/**
 * Implementation of interface with the Aggregation Prover through Files.
 *
 * Aggregation Prover will ingest file like
 * path/to/prover/requests/<startBlockNumber>-<endBlockNumber>-<requestHash>-getZkAggregatedProof.json
 *
 * When done prover will output file
 * path/to/prover/responses/<startBlockNumber>-<endBlockNumber>-<requestHash>-getZkAggregatedProof.json
 *
 * So, this class will need to watch the file system and wait for the output proof to be generated
 */
class FileBasedProofAggregationClientV2(
  vertx: Vertx,
  val config: FileBasedProverConfig,
  val invalidityProverConfig: FileBasedProverConfig? = null,
  hashFunction: HashFunction = Sha256HashFunction(),
  executionProofResponseFileNameProvider: ProverFileNameProvider<ExecutionProofIndex> =
    ExecutionProofFileNameProvider,
  compressionProofResponseFileNameProvider: ProverFileNameProvider<CompressionProofIndex> =
    CompressionProofResponseFileNameProvider,
  jsonObjectMapper: ObjectMapper = JsonSerialization.proofResponseMapperV1,
  log: Logger,
) :
  GenericFileBasedProverClient<
    ProofsToAggregate,
    ProofToFinalize,
    AggregationProofRequestDto,
    ProofToFinalizeJsonResponse,
    AggregationProofIndex,
    >(
    config = config,
    vertx = vertx,
    fileWriter = FileWriter(vertx, jsonObjectMapper),
    fileReader = FileReader(
      vertx,
      jsonObjectMapper,
      ProofToFinalizeJsonResponse::class.java,
    ),
    requestFileNameProvider = AggregationProofFileNameProvider,
    responseFileNameProvider = AggregationProofFileNameProvider,
    proofIndexProvider = createProofIndexProviderFn(hashFunction),
    requestMapper = AggregationRequestDtoMapper(
      executionProofResponseFileNameProvider = executionProofResponseFileNameProvider,
      compressionProofResponseFileNameProvider = compressionProofResponseFileNameProvider,
      invalidityProofResponseFileNameProvider = InvalidityProofFileNameProvider,
    ),
    responseMapper = ProofToFinalizeJsonResponse::toDomainObject,
    proofTypeLabel = "aggregation",
    log = log,
  ),
  ProofAggregationProverClientV2 {

  override fun createProofRequest(proofRequest: ProofsToAggregate): SafeFuture<AggregationProofIndex> {
    return awaitInvalidityProofResponses(proofRequest).thenCompose {
      super.createProofRequest(proofRequest)
    }
  }

  private fun awaitInvalidityProofResponses(
    proofRequest: ProofsToAggregate,
  ): SafeFuture<Unit> {
    if (proofRequest.invalidityProofs.isEmpty()) {
      return SafeFuture.completedFuture(Unit)
    }
    if (invalidityProverConfig == null) {
      throw IllegalStateException("Proof request contains invalidity proofs but invalidity Prover is not configured")
    }

    val responseNames = proofRequest.invalidityProofs.map { invalidityProofIndex ->
      val responseFileName = InvalidityProofFileNameProvider.getFileName(invalidityProofIndex)
      invalidityProverConfig.responsesDirectory.resolve(responseFileName)
    }
    return fileMonitor
      .awaitForAllFiles(responseNames)
      .thenApply { null }
  }

  companion object {
    val LOG: Logger = LogManager.getLogger(FileBasedProofAggregationClientV2::class.java)

    fun createProofIndexProviderFn(
      hashFunction: HashFunction,
      executionProofResponseFileNameProvider: ProverFileNameProvider<ExecutionProofIndex> =
        ExecutionProofFileNameProvider,
      compressionProofResponseFileNameProvider: ProverFileNameProvider<CompressionProofIndex> =
        CompressionProofResponseFileNameProvider,
    ): (ProofsToAggregate) -> AggregationProofIndex {
      return { request: ProofsToAggregate ->

        val requestDto = AggregationProofRequestDto.fromDomainObject(
          proofsToAggregate = request,
          executionProofResponseFileNameProvider = executionProofResponseFileNameProvider,
          compressionProofResponseFileNameProvider = compressionProofResponseFileNameProvider,
          invalidityProofResponseFileNameProvider = InvalidityProofFileNameProvider,
        )
        val hash = hashRequest(hashFunction, requestDto)
        AggregationProofIndex(
          startBlockNumber = request.startBlockNumber,
          endBlockNumber = request.endBlockNumber,
          hash = hash,
        )
      }
    }

    private fun hashRequest(hashFunction: HashFunction, request: AggregationProofRequestDto): ByteArray {
      val proofFileNames = (request.compressionProofs + request.executionProofs)
      val contentBytes = proofFileNames.joinToString().toByteArray()
      val hash = hashFunction.hash(contentBytes)
      LOG.trace(
        "Hash Aggregation request={}, proofFileNames={}, contentBytes={}, hash={}, hashFunction={}",
        request,
        proofFileNames.joinToString(),
        contentBytes.encodeHex(),
        hash.encodeHex(),
        hashFunction.toString(),
      )
      return hash
    }
  }
}
