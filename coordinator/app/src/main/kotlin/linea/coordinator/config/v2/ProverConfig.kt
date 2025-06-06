package linea.coordinator.config.v2

import kotlin.time.Duration
import kotlin.time.Duration.Companion.seconds

data class ProverConfig(
  val version: String,
  val fsInprogressRequestWritingSuffix: String = ".inprogress_coordinator_writing",
  val fsInprogressProvingSuffixPattern: String = "\\.inprogress\\.prover.*",
  val fsPollingInterval: Duration = 15.seconds,
  val fsPollingTimeout: Duration? = null,
  val execution: ProverDirectoriesToml,
  val blobCompression: ProverDirectoriesToml,
  val proofAggregation: ProverDirectoriesToml,
  val new: ProverConfig? = null,
) {
  data class ProverDirectoriesToml(
    val fsRequestsDirectory: String,
    val fsResponsesDirectory: String,
  )
}
