package net.consensys.linea

import com.github.michaelbull.result.Result
import com.github.michaelbull.result.flatMap
import linea.domain.BlockNumberAndHash
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture

class TracesCountingServiceWithOriginalJsonCounter(
  private val repository: TracesRepositoryV1,
  private val tracesCounterV0: TracesCounterV0
) : TracesCountingServiceV1 {
  val log: Logger = LogManager.getLogger(this::class.java)

  override fun getBlockTracesCounters(
    block: BlockNumberAndHash,
    version: String
  ): SafeFuture<Result<VersionedResult<BlockCounters>, TracesError>> {
    val tracesFileIndex = TracesFileIndex(block, version)
    return repository.getTraces(listOf(tracesFileIndex)).thenApply { result ->
      result.flatMap { blocksTraces ->
        tracesCounterV0.countTraces(blocksTraces[0].traces)
      }
    }
  }
}

class TracesCountingServiceWithEfficientStringParserCounter(
  private val repository: TracesRepositoryV1,
  private val tracesCounter: TracesCounter
) : TracesCountingServiceV1 {
  val log: Logger = LogManager.getLogger(this::class.java)

  override fun getBlockTracesCounters(
    block: BlockNumberAndHash,
    version: String
  ): SafeFuture<Result<VersionedResult<BlockCounters>, TracesError>> {
    val tracesFileIndex = TracesFileIndex(block, version)

    return repository.getTracesAsString(tracesFileIndex).thenApply { result ->
      result.flatMap { blockTraces -> tracesCounter.countTraces(blockTraces) }
    }
  }
}

class TracesCountingServiceWithRetry(
  private val efficientCounter: TracesCountingServiceV1,
  private val jsonOriginalCounter: TracesCountingServiceV1
) : TracesCountingServiceV1 {
  val log: Logger = LogManager.getLogger(this::class.java)
  constructor(
    repository: TracesRepositoryV1,
    tracesCounter: TracesCounter,
    tracesCounterV0: TracesCounterV0
  ) : this(
    efficientCounter = TracesCountingServiceWithEfficientStringParserCounter(repository, tracesCounter),
    jsonOriginalCounter = TracesCountingServiceWithOriginalJsonCounter(repository, tracesCounterV0)
  )

  override fun getBlockTracesCounters(
    block: BlockNumberAndHash,
    version: String
  ): SafeFuture<Result<VersionedResult<BlockCounters>, TracesError>> {
    return efficientCounter.getBlockTracesCounters(block, version)
      .exceptionallyCompose { e ->
        if (e.cause is OutOfMemoryError) {
          // parsing the whole file as a string failed, try to parse it as a json object.
          // Less performant but does not try to lead the whole file into a single String, limited to 2^31-1 bytes
          jsonOriginalCounter.getBlockTracesCounters(block, version)
        } else {
          log.error("Error getting traces for block={} error={}", block.number, e.message, e)
          SafeFuture.failedFuture(e)
        }
      }
  }
}
