package lineth.conflation.calculators

import linea.da.DaStreamPosition
import linea.da.blobBytes
import linea.domain.BlocksConflation
import lineth.conflation.calculators.DaTestConflations.encoder
import lineth.conflation.calculators.DaTestConflations.genesisDataRollingHash
import lineth.coordination.riscv.da.FakeDaStream
import kotlin.time.Duration

/** The RISC-V blob and rollup calculators wired together on [FakeDaStream], for pipeline tests. */
object DaPipelineTestSupport {
  /** Blob and rollup calculators wired together on a [FakeDaStream] opened at [position], collecting the rollups. */
  class Pipeline(
    position: DaStreamPosition = DaStreamPosition(genesisDataRollingHash),
    rollupTriggers: List<RollupTriggerCalculator> = emptyList(),
    blobTriggers: List<BlobTriggerCalculator> = emptyList(),
    rollupDeadline: Duration? = null,
  ) {
    val rollups = mutableListOf<Rollup>()
    val rollupCalculator: GlobalRollupCalculator = GlobalRollupCalculator(
      triggers = rollupTriggers,
      openRollup = RollupDaAssembler(position),
      blobCalculator = { this.blobCalculator },
      rollupDeadline = rollupDeadline,
    ).also { it.onRollup { rollup -> rollups += rollup } }
    val blobCalculator: GlobalBlobCalculator = GlobalBlobCalculator(
      daStream = FakeDaStream(position),
      triggers = blobTriggers,
      blockEncoder = encoder,
      handler = rollupCalculator,
      sharedBlobTailBytes = position.tailBytes,
    )

    fun feed(vararg conflations: BlocksConflation) = conflations.forEach(blobCalculator::newConflation)
  }

  /** The stream position at the end of [rollup], as restart would rebuild it from the rollup's persisted row. */
  fun positionAfter(rollup: Rollup): DaStreamPosition {
    val last = rollup.da.blobs.last()
    val tail = rollup.da.expectedFinalDataTailDiscardBytes
    return DaStreamPosition(
      dataRollingHash = rollup.da.expectedEndDataRollingHash,
      tailBytes = tail,
      sharedBlob = last.blobBytes().takeIf { tail > 0 },
      prevDataRollingHash = last.prevDataRollingHash.takeIf { tail > 0 },
    )
  }
}
