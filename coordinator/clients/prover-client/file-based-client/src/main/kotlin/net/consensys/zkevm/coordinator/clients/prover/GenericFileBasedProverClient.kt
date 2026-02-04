package net.consensys.zkevm.coordinator.clients.prover

import com.github.michaelbull.result.Err
import com.github.michaelbull.result.getOrElse
import com.github.michaelbull.result.map
import io.vertx.core.Vertx
import net.consensys.linea.errors.ErrorResponse
import net.consensys.zkevm.coordinator.clients.ProverProofRequestCreator
import net.consensys.zkevm.coordinator.clients.ProverProofResponseChecker
import net.consensys.zkevm.domain.ProofIndex
import net.consensys.zkevm.fileio.FileMonitor
import net.consensys.zkevm.fileio.FileReader
import net.consensys.zkevm.fileio.FileWriter
import net.consensys.zkevm.fileio.inProgressFilePattern
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.nio.file.Path
import java.util.concurrent.atomic.AtomicLong
import java.util.function.Supplier
import kotlin.io.path.notExists

open class GenericFileBasedProverClient<Request, Response, RequestDto, ResponseDto, TProofIndex>(
  private val config: FileBasedProverConfig,
  private val vertx: Vertx,
  private val fileWriter: FileWriter,
  private val fileReader: FileReader<ResponseDto>,
  private val requestFileNameProvider: ProverFileNameProvider<TProofIndex>,
  private val responseFileNameProvider: ProverFileNameProvider<TProofIndex>,
  private val fileMonitor: FileMonitor = FileMonitor(
    vertx,
    FileMonitor.Config(config.pollingInterval, config.pollingTimeout),
  ),
  private val proofIndexProvider: (Request) -> TProofIndex,
  private val requestMapper: (Request) -> SafeFuture<RequestDto>,
  private val responseMapper: (ResponseDto) -> Response,
  private val proofTypeLabel: String,
  private val log: Logger = LogManager.getLogger(GenericFileBasedProverClient::class.java),
) : ProverProofResponseChecker<Response, TProofIndex>, Supplier<Number>, ProverProofRequestCreator<Request, TProofIndex>
  where TProofIndex : ProofIndex, Request : Any, RequestDto : Any {

  init {
    createDirectoryIfNotExists(config.requestsDirectory, log)
    createDirectoryIfNotExists(config.responsesDirectory, log)
  }

  private val responsesWaiting = AtomicLong(0)
  override fun get(): Long = responsesWaiting.get()

  fun isResponseAlreadyDone(proofIndex: TProofIndex): SafeFuture<Path?> {
    val responseFilePath = config.responsesDirectory.resolve(responseFileNameProvider.getFileName(proofIndex))
    return fileMonitor
      .fileExists(responseFilePath)
      .thenApply { responseFileExists ->
        if (responseFileExists) {
          log.debug(
            "request already proven: {}={} reusedResponse={}",
            proofTypeLabel,
            proofIndex,
            responseFilePath,
          )
          responseFilePath
        } else {
          null
        }
      }
  }

  override fun findProofResponse(proofIndex: TProofIndex): SafeFuture<Response?> {
    return isResponseAlreadyDone(proofIndex)
      .thenCompose { responseFilePath ->
        responseFilePath
          ?.let { parseResponse(it, proofIndex) }
          ?: SafeFuture.completedFuture(null)
      }
  }

  override fun createProofRequest(proofRequest: Request): SafeFuture<TProofIndex> {
    val proofIndex = proofIndexProvider(proofRequest)
    val requestFileName = requestFileNameProvider.getFileName(proofIndex)
    val requestFilePath = config.requestsDirectory.resolve(requestFileName)

    return findRequestFileIfAlreadyInFileSystem(requestFileName)
      .thenCompose { requestFileFound: String? ->
        if (requestFileFound != null) {
          log.debug(
            "request already in file system: {}={} reusedRequest={}",
            proofTypeLabel,
            proofIndex,
            requestFileFound,
          )
          SafeFuture.completedFuture(proofIndex)
        } else {
          requestMapper(proofRequest)
            .thenCompose { proofRequestDto ->
              fileWriter.write(
                proofRequestDto,
                requestFilePath,
                config.inprogressRequestWritingSuffix,
              ).thenApply {
                proofIndex
              }
            }
        }
      }
  }

  fun requestProof(proofRequest: Request): SafeFuture<Response> {
    val proofIndex = proofIndexProvider(proofRequest)
    val responseFilePath = config.responsesDirectory.resolve(responseFileNameProvider.getFileName(proofIndex))

    return findProofResponse(proofIndex)
      .thenCompose { response ->
        if (response != null) {
          SafeFuture.completedFuture(response)
        } else {
          responsesWaiting.incrementAndGet()
          createProofRequest(proofRequest)
            .thenCompose { waitForResponse(responseFilePath) }
            .thenCompose {
              responsesWaiting.decrementAndGet()
              parseResponse(responseFilePath, proofIndex)
            }
        }
      }
      .whenException {
        log.error(
          "Failed to get proof: {}={} errorMessage={}",
          proofTypeLabel,
          proofIndex,
          it.message,
          it,
        )
      }
  }

  private fun waitForResponse(responseFilePath: Path): SafeFuture<Path> {
    return fileMonitor.monitor(responseFilePath).thenCompose {
      if (it is Err) {
        when (it.error) {
          FileMonitor.ErrorType.TIMED_OUT -> {
            SafeFuture.failedFuture<Path>(RuntimeException("Timeout waiting for response file=$responseFilePath"))
          }
          // else -> {
          //  SafeFuture.failedFuture(RuntimeException("Unexpected error=$it"))
          // }
        }
      } else {
        SafeFuture.completedFuture(responseFilePath)
      }
    }
  }

  private fun findRequestFileIfAlreadyInFileSystem(requestFileName: String): SafeFuture<String?> {
    return fileMonitor.findFile(
      directory = config.requestsDirectory,
      pattern = inProgressFilePattern(requestFileName, config.inprogressProvingSuffixPattern),
    )
  }

  protected open fun parseResponse(responseFilePath: Path, proofIndex: TProofIndex): SafeFuture<Response> {
    return fileReader.read(responseFilePath)
      .thenCompose { result ->
        result
          .map { SafeFuture.completedFuture(responseMapper(it)) }
          .getOrElse { errorResponse: ErrorResponse<FileReader.ErrorType> ->
            when (errorResponse.type) {
              FileReader.ErrorType.PARSING_ERROR -> {
                log.error(
                  "Failed to read response file={} errorMessage={}",
                  responseFilePath,
                  errorResponse.message,
                )
              }
            }
            SafeFuture.failedFuture(errorResponse.asException())
          }
      }
  }

  companion object {
    fun createDirectoryIfNotExists(
      directory: Path,
      log: Logger = LogManager.getLogger(GenericFileBasedProverClient::class.java),
    ) {
      try {
        if (directory.notExists()) {
          val dirCreated = directory.toFile().mkdirs()
          if (!dirCreated) {
            log.error("Failed to create directory {}!", directory)
            throw RuntimeException("Failed to create directory $directory")
          }
        }
      } catch (e: Exception) {
        log.error("Failed to create directory {}!", directory, e)
        throw e
      }
    }
  }
}
