package linea.coordinator.clients.prover.riscv

import com.github.michaelbull.result.Err
import com.github.michaelbull.result.getOrElse
import com.github.michaelbull.result.map
import io.vertx.core.Vertx
import linea.coordinator.clients.prover.FileBasedProverConfig
import linea.coordinator.clients.prover.GenericFileBasedProverClient
import linea.coordinator.clients.prover.ProverFileNameProvider
import linea.domain.ProofIndex
import linea.error.ErrorResponse
import linea.fileio.FileMonitor
import linea.fileio.FileReader
import linea.fileio.FileWriter
import linea.fileio.inProgressFilePattern
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.nio.file.Path

/**
 * File-based [ProverProofTransport]: the request DTO is written as a JSON file into [FileBasedProverConfig.requestsDirectory]
 * and the response is read from a JSON file the prover writes into [FileBasedProverConfig.responsesDirectory]. This
 * reproduces the behaviour of `GenericFileBasedProverClient` behind the transport abstraction.
 */
class FileBasedProverProofTransport<RequestDto : Any, ResponseDto, TProofIndex : ProofIndex>(
  private val config: FileBasedProverConfig,
  vertx: Vertx,
  private val fileWriter: FileWriter,
  private val fileReader: FileReader<ResponseDto>,
  private val requestFileNameProvider: ProverFileNameProvider<TProofIndex>,
  private val responseFileNameProvider: ProverFileNameProvider<TProofIndex>,
  private val fileMonitor: FileMonitor = FileMonitor(
    vertx,
    FileMonitor.Config(config.pollingInterval, config.pollingTimeout),
  ),
  private val log: Logger = LogManager.getLogger(FileBasedProverProofTransport::class.java),
) : ProverProofTransport<RequestDto, ResponseDto, TProofIndex> {

  init {
    GenericFileBasedProverClient.createDirectoryIfNotExists(config.requestsDirectory, log)
    GenericFileBasedProverClient.createDirectoryIfNotExists(config.responsesDirectory, log)
  }

  private fun responseFilePath(proofIndex: TProofIndex): Path =
    config.responsesDirectory.resolve(responseFileNameProvider.getFileName(proofIndex))

  override fun isRequestAlreadySubmitted(proofIndex: TProofIndex): SafeFuture<Boolean> {
    val requestFileName = requestFileNameProvider.getFileName(proofIndex)
    return fileMonitor.findFile(
      directory = config.requestsDirectory,
      pattern = inProgressFilePattern(requestFileName, config.inprogressProvingSuffixPattern),
    ).thenApply { it != null }
  }

  override fun submitRequest(proofIndex: TProofIndex, requestDto: RequestDto): SafeFuture<Unit> {
    val requestFilePath = config.requestsDirectory.resolve(requestFileNameProvider.getFileName(proofIndex))
    log.trace("Creating proof request file. file={}", requestFilePath)
    return fileWriter.write(requestDto, requestFilePath, config.inprogressRequestWritingSuffix)
      .thenApply {
        log.trace("Created proof request file. file={}", requestFilePath)
        Unit
      }
  }

  override fun findResponse(proofIndex: TProofIndex): SafeFuture<ResponseDto?> {
    val responseFilePath = responseFilePath(proofIndex)
    return fileMonitor.fileExists(responseFilePath)
      .thenCompose { exists ->
        if (exists) {
          parseResponse(responseFilePath).thenApply { it }
        } else {
          SafeFuture.completedFuture(null)
        }
      }
  }

  override fun awaitResponse(proofIndex: TProofIndex): SafeFuture<ResponseDto> {
    val responseFilePath = responseFilePath(proofIndex)
    return fileMonitor.monitor(responseFilePath)
      .thenCompose { result ->
        if (result is Err) {
          when (result.error) {
            FileMonitor.ErrorType.TIMED_OUT ->
              SafeFuture.failedFuture(RuntimeException("Timeout waiting for response file=$responseFilePath"))
          }
        } else {
          parseResponse(responseFilePath)
        }
      }
  }

  private fun parseResponse(responseFilePath: Path): SafeFuture<ResponseDto> {
    return fileReader.read(responseFilePath)
      .thenCompose { result ->
        result
          .map { SafeFuture.completedFuture(it) }
          .getOrElse { errorResponse: ErrorResponse<FileReader.ErrorType> ->
            when (errorResponse.type) {
              FileReader.ErrorType.PARSING_ERROR ->
                log.error("Failed to read response file={} errorMessage={}", responseFilePath, errorResponse.message)
            }
            SafeFuture.failedFuture(errorResponse.asException())
          }
      }
  }
}
