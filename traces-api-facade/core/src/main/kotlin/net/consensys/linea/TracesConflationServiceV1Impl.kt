package net.consensys.linea

import com.github.michaelbull.result.Err
import com.github.michaelbull.result.Ok
import com.github.michaelbull.result.Result
import com.github.michaelbull.result.flatMap
import io.vertx.core.json.JsonObject
import linea.domain.BlockNumberAndHash
import linea.domain.CommonDomainFunctions.blockIntervalString
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture

class TracesConflationServiceV1Impl(
  private val repository: TracesRepositoryV1,
  private val tracesConflator: TracesConflator,
  private val conflatedTracesRepository: ConflatedTracesRepository,
  private val tracesVersion: String
) : TracesConflationServiceV1 {
  val log: Logger = LogManager.getLogger(this::class.java)
  override fun getConflatedTraces(
    blocks: List<BlockNumberAndHash>,
    version: String
  ): SafeFuture<Result<VersionedResult<JsonObject>, TracesError>> {
    val tracesIndexes = blocks.map {
      TracesFileIndex(it, version)
    }
    return repository.getTraces(tracesIndexes)
      .thenApply { result ->
        result.flatMap { blocksTraces ->
          tracesConflator.conflateTraces(blocksTraces.sortedBy { it.blockNumber }.map { it.traces })
        }
      }
  }

  override fun generateConflatedTracesToFile(
    blocks: List<BlockNumberAndHash>,
    version: String
  ): SafeFuture<Result<VersionedResult<String>, TracesError>> {
    val blocksSorted = blocks.sortedBy { it.number }
    // we check if we already have the conflation for the given blocks
    return conflatedTracesRepository.findConflatedTraces(
      blocksSorted.first().number,
      blocksSorted.last().number,
      tracesVersion
    ).thenCompose { conflatedTracesFileName: String? ->
      if (conflatedTracesFileName != null) {
        log.info(
          "Reusing conflated traces for batch={} file={}",
          blockIntervalString(blocksSorted.first().number, blocksSorted.last().number),
          conflatedTracesFileName
        )
        SafeFuture.completedFuture(Ok(VersionedResult(tracesVersion, conflatedTracesFileName)))
      } else {
        getConflatedTraces(blocks, version).thenCompose { result ->
          when (result) {
            is Ok -> {
              conflatedTracesRepository
                .saveConflatedTraces(
                  TracesConflation(blocksSorted.first().number, blocksSorted.last().number, result.value)
                )
                .thenApply { Ok(VersionedResult(result.value.version, it)) }
            }

            is Err -> SafeFuture.completedFuture(Err(result.error))
          }
        }
      }
    }
  }
}
