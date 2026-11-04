package lineth.conflation.calculators

import linea.da.DaStreamPosition
import linea.da.blobBytes
import linea.domain.BlocksConflation
import linea.domain.ConflationCalculationResult
import linea.domain.ConflationTrigger
import linea.domain.createBlock
import linea.domain.toBesu
import linea.rlp.RLP
import lineth.coordination.riscv.da.FakeDaStream
import lineth.encoding.BlockEncoder
import net.consensys.linea.traces.TracesCountersV5
import kotlin.time.Duration
import kotlin.time.Duration.Companion.seconds
import kotlin.time.Instant

/** Helpers for tests of the RISC-V blob and rollup calculators on [FakeDaStream]. */
object DaPipelineTestSupport {
  val encoder = BlockEncoder { RLP.encodeBlock(it.toBesu()) }
  val baseTime: Instant = Instant.parse("2026-10-01T00:00:00Z")
  val genesisDataRollingHash = ByteArray(32) { 0x11 }

  /** One-block conflation whose DA frame is exactly [frameSize] bytes. */
  fun conflation(
    number: ULong,
    frameSize: Int,
    timestamp: Instant = baseTime + (number.toLong() * 2).seconds,
    gasUsed: ULong = 1_000UL,
  ): BlocksConflation {
    var extraDataSize = frameSize
    repeat(10) {
      val block = createBlock(
        number = number,
        timestamp = timestamp,
        gasUsed = gasUsed,
        extraData = ByteArray(extraDataSize) { 0x2a },
      )
      val size = Int.SIZE_BYTES + encoder.encode(block).size
      if (size == frameSize) {
        return BlocksConflation(
          listOf(block),
          ConflationCalculationResult(
            number,
            number,
            ConflationTrigger.BLOCKS_LIMIT,
            TracesCountersV5.EMPTY_TRACES_COUNT,
          ),
        )
      }
      extraDataSize += frameSize - size
    }
    error("could not build a frame of $frameSize bytes")
  }

  /** Blob and rollup calculators wired together on a [FakeDaStream] opened at [position], collecting the rollups. */
  class Pipeline(
    position: DaStreamPosition = DaStreamPosition(genesisDataRollingHash),
    rollupTriggers: List<RollupTriggerCalculator> = emptyList(),
    blobTriggers: List<BlobTriggerCalculator> = emptyList(),
    rollupDeadline: Duration? = null,
  ) {
    val rollups = mutableListOf<Rollup>()
    val rollupCalculator: GlobalRollupCalculatorV2 = GlobalRollupCalculatorV2(
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
      resumingInsideSharedBlob = position.tailBytes > 0,
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
