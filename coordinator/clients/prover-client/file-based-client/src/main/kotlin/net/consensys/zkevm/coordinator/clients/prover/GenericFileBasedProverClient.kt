package net.consensys.zkevm.coordinator.clients.prover

import com.github.michaelbull.result.Err
import com.github.michaelbull.result.getOrElse
import com.github.michaelbull.result.map
import io.vertx.core.Vertx
import linea.domain.BlockInterval
import net.consensys.linea.errors.ErrorResponse
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

open class GenericFileBasedProverClient<Request, Response, RequestDto, ResponseDto>(
  private val config: FileBasedProverConfig,
  private val vertx: Vertx,
  private val fileWriter: FileWriter,
  private val fileReader: FileReader<ResponseDto>,
  private val requestFileNameProvider: ProverFileNameProvider,
  private val responseFileNameProvider: ProverFileNameProvider,
  private val fileMonitor: FileMonitor = FileMonitor(
    vertx,
    FileMonitor.Config(config.pollingInterval, config.pollingTimeout)
  ),
  private val proofIndexProvider: (Request) -> ProofIndex = ::blockIntervalProofIndex,
  private val requestMapper: (Request) -> SafeFuture<RequestDto>,
  private val responseMapper: (ResponseDto) -> Response,
  private val proofTypeLabel: String,
  private val log: Logger = LogManager.getLogger(GenericFileBasedProverClient::class.java)
) : Supplier<Number>
  where Request : BlockInterval,
        Response : Any,
        RequestDto : Any,
        ResponseDto : Any {

  init {
    createDirectoryIfNotExists(config.requestsDirectory, log)
    createDirectoryIfNotExists(config.responsesDirectory, log)
  }

  private val responsesWaiting = AtomicLong(0)
  override fun get(): Long = responsesWaiting.get()

  fun requestProof(proofRequest: Request): SafeFuture<Response> {
    val proofIndex = proofIndexProvider(proofRequest)
    val requestFileName = requestFileNameProvider.getFileName(proofIndex)
    val requestFilePath = config.requestsDirectory.resolve(requestFileName)
    val responseFilePath = config.responsesDirectory.resolve(responseFileNameProvider.getFileName(proofIndex))

    return fileMonitor.fileExists(responseFilePath)
      .thenCompose { responseFileExists ->
        if (responseFileExists) {
          log.debug(
            "request already proven: {}={} reusedResponse={}",
            proofTypeLabel,
            proofIndex.intervalString(),
            responseFilePath
          )
          SafeFuture.completedFuture(responseFilePath)
        } else {
          findRequestFileIfAlreadyInFileSystem(requestFileName)
            .thenCompose { requestFileFound: String? ->
              responsesWaiting.incrementAndGet()
              if (requestFileFound != null) {
                log.debug(
                  "request already in file system: {}={} reusedRequest={}",
                  proofTypeLabel,
                  proofIndex.intervalString(),
                  requestFileFound
                )
                SafeFuture.completedFuture(Unit)
              } else {
                requestMapper(proofRequest)
                  .thenCompose { proofRequestDto ->
                    fileWriter.write(
                      proofRequestDto,
                      requestFilePath,
                      config.inprogressRequestWritingSuffix
                    ).thenApply {
                      Unit
                    }
                  }
              }
            }
            .thenCompose { waitForResponse(responseFilePath) }
            .thenApply {
              responsesWaiting.decrementAndGet()
              responseFilePath
            }
        }
      }
      .thenCompose { proofResponseFilePath -> parseResponse(proofResponseFilePath, proofIndex) }
      .whenException {
        log.error(
          "Failed to get proof: {}={} errorMessage={}",
          proofTypeLabel,
          proofIndex.intervalString(),
          it.message,
          it
        )
      }
  }

  private fun waitForResponse(
    responseFilePath: Path
  ): SafeFuture<Path> {
    return fileMonitor.monitor(responseFilePath).thenCompose {
      if (it is Err) {
        when (it.error) {
          FileMonitor.ErrorType.TIMED_OUT -> {
            SafeFuture.failedFuture<Path>(RuntimeException("Timeout waiting for response file=$responseFilePath"))
          }

          else -> {
            SafeFuture.failedFuture(RuntimeException("Unexpected error=$it"))
          }
        }
      } else {
        SafeFuture.completedFuture(responseFilePath)
      }
    }
  }

  private fun findRequestFileIfAlreadyInFileSystem(
    requestFileName: String
  ): SafeFuture<String?> {
    return fileMonitor.findFile(
      directory = config.requestsDirectory,
      pattern = inProgressFilePattern(requestFileName, config.inprogressProvingSuffixPattern)
    )
  }

  protected open fun parseResponse(
    responseFilePath: Path,
    proofIndex: ProofIndex
  ): SafeFuture<Response> {
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
                  errorResponse.message
                )
              }
            }
            SafeFuture.failedFuture(errorResponse.asException())
          }
      }
  }

  companion object {
    fun <R : BlockInterval> blockIntervalProofIndex(request: R): ProofIndex {
      return ProofIndex(
        startBlockNumber = request.startBlockNumber,
        endBlockNumber = request.endBlockNumber
      )
    }

    fun createDirectoryIfNotExists(
      directory: Path,
      log: Logger = LogManager.getLogger(GenericFileBasedProverClient::class.java)
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
