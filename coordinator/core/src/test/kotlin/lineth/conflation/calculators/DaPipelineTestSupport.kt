package lineth.conflation.calculators

import linea.da.DaStreamPosition
import linea.domain.BlocksConflation
import lineth.conflation.calculators.DaTestConflations.encoder
import lineth.conflation.calculators.DaTestConflations.genesisDataRollingHash
import lineth.coordination.riscv.da.FakeDaCompressor
import lineth.coordination.riscv.da.FakeDaStream
import kotlin.time.Duration

/** The RISC-V blob and rollup calculators wired together on [FakeDaStream], for pipeline tests. */
object DaPipelineTestSupport {
  /** Blob and rollup calculators wired together on a [FakeDaStream] opened at [position], collecting the rollups. */
  class Pipeline(
    position: DaStreamPosition = DaStreamPosition.fresh(genesisDataRollingHash),
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
      compressor = FakeDaCompressor(),
      daStream = FakeDaStream(position),
      triggers = blobTriggers,
      blockEncoder = encoder,
      handler = rollupCalculator,
      sharedBlobCarryOverTailBytes = position.sharedBlobCarryOverTailBytes,
    )

    fun feed(vararg conflations: BlocksConflation) = conflations.forEach(blobCalculator::newConflation)
  }

  /** The stream position at the end of [rollup], as restart would rebuild it from the rollup's persisted row. */
  fun positionAfter(rollup: Rollup): DaStreamPosition {
    val last = rollup.da.blobs.last()
    val tail = rollup.da.expectedFinalDataTailDiscardBytes
    if (tail == 0u) return DaStreamPosition.fresh(rollup.da.expectedEndDataRollingHash)
    return DaStreamPosition(
      dataRollingHash = rollup.da.expectedEndDataRollingHash,
      parentBlobDataRollingHash = last.prevDataRollingHash,
      sharedBlobCarryOverTailBytes = tail,
      sharedBlob = last.blob,
    )
  }
}
