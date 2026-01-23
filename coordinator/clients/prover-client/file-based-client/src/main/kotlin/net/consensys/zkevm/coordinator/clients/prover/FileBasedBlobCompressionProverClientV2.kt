package net.consensys.zkevm.coordinator.clients.prover

import com.fasterxml.jackson.databind.ObjectMapper
import io.vertx.core.Vertx
import net.consensys.zkevm.coordinator.clients.BlobCompressionProof
import net.consensys.zkevm.coordinator.clients.BlobCompressionProofRequest
import net.consensys.zkevm.coordinator.clients.BlobCompressionProverClientV2
import net.consensys.zkevm.coordinator.clients.prover.serialization.BlobCompressionProofJsonRequest
import net.consensys.zkevm.coordinator.clients.prover.serialization.BlobCompressionProofJsonResponse
import net.consensys.zkevm.coordinator.clients.prover.serialization.JsonSerialization
import net.consensys.zkevm.domain.ProofIndex
import net.consensys.zkevm.fileio.FileReader
import net.consensys.zkevm.fileio.FileWriter
import org.apache.logging.log4j.LogManager
import tech.pegasys.teku.infrastructure.async.SafeFuture

/**
 * Implementation of interface with the Blob Compression Prover through Files.
 *
 * Blob Compression Prover will ingest file like
 * path/to/prover/requests/<startBlockNumber>-<endBlockNumber>-<expectedShnarf>-getZkBlobCompressionProof.json
 *
 * When done prover will output file
 * path/to/prover/responses/<startBlockNumber>-<endBlockNumber>-<expectedShnarf>-getZkBlobCompressionProof.json
 *
 * So, this class will need to watch the file system and wait for the output proof to be generated
 */
class FileBasedBlobCompressionProverClientV2(
  val config: FileBasedProverConfig,
  val vertx: Vertx,
  jsonObjectMapper: ObjectMapper = JsonSerialization.proofResponseMapperV1,
) :
  GenericFileBasedProverClient<
    BlobCompressionProofRequest,
    BlobCompressionProof,
    BlobCompressionProofJsonRequest,
    BlobCompressionProofJsonResponse,
    >(
    config = config,
    vertx = vertx,
    fileWriter = FileWriter(vertx, jsonObjectMapper),
    fileReader = FileReader(
      vertx,
      jsonObjectMapper,
      BlobCompressionProofJsonResponse::class.java,
    ),
    requestFileNameProvider = CompressionProofRequestFileNameProvider,
    responseFileNameProvider = CompressionProofResponseFileNameProvider,
    proofIndexProvider = FileBasedBlobCompressionProverClientV2::blobFileIndex,
    requestMapper = FileBasedBlobCompressionProverClientV2::requestDtoMapper,
    responseMapper = BlobCompressionProofJsonResponse::toDomainObject,
    proofTypeLabel = "blob",
    log = LogManager.getLogger(FileBasedBlobCompressionProverClientV2::class.java),
  ),
  BlobCompressionProverClientV2 {

  companion object {
    fun blobFileIndex(request: BlobCompressionProofRequest): ProofIndex {
      return ProofIndex(
        startBlockNumber = request.startBlockNumber,
        endBlockNumber = request.endBlockNumber,
        hash = request.expectedShnarfResult.expectedShnarf,
      )
    }

    fun requestDtoMapper(domainRequest: BlobCompressionProofRequest): SafeFuture<BlobCompressionProofJsonRequest> {
      return SafeFuture.completedFuture(BlobCompressionProofJsonRequest.fromDomainObject(domainRequest))
    }
  }
}
