package net.consensys.zkevm.coordinator.clients.prover

import java.nio.file.Path
import kotlin.time.Duration

data class ProversConfig(
  val proverA: ProverConfig,
  val switchBlockNumberInclusive: ULong?,
  val proverB: ProverConfig?,
  val enableRequestFilesCleanup: Boolean = false,
)

data class ProverConfig(
  val execution: FileBasedProverConfig,
  val invalidity: FileBasedProverConfig? = null,
  val blobCompression: FileBasedProverConfig,
  val proofAggregation: FileBasedProverConfig,
)

data class FileBasedProverConfig(
  val requestsDirectory: Path,
  val responsesDirectory: Path,
  val inprogressProvingSuffixPattern: String,
  val inprogressRequestWritingSuffix: String,
  val pollingInterval: Duration,
  val pollingTimeout: Duration,
)
