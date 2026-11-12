package lineth.conflation.calculators

import linea.domain.BlocksConflation
import linea.domain.ConflationCalculationResult
import linea.domain.ConflationTrigger
import linea.domain.createBlock
import linea.domain.toBesu
import linea.rlp.RLP
import lineth.coordination.riscv.da.FakeDaStream
import lineth.encoding.BlockEncoder
import net.consensys.linea.traces.TracesCountersV5
import kotlin.time.Duration.Companion.seconds
import kotlin.time.Instant

/** Conflations with an exact DA frame size, for tests of the RISC-V blob and rollup calculators on [FakeDaStream]. */
object DaTestConflations {
  val encoder = BlockEncoder { RLP.encodeBlock(it.toBesu()) }
  val baseTime: Instant = Instant.parse("2026-10-01T00:00:00Z")
  val genesisDataRollingHash = ByteArray(32) { 0x11 }

  /** One-block conflation whose DA frame is exactly [frameSize] bytes. */
  fun conflation(
    number: ULong,
    frameSize: Int,
    timestamp: Instant = baseTime + (number.toLong() * 2).seconds,
    gasUsed: ULong = 1_000UL,
  ): BlocksConflation = conflation(number, number, frameSize, timestamp, gasUsed)

  /**
   * Conflation of blocks [startNumber]..[endNumber], 2 seconds apart from [firstTimestamp] and each using [gasUsed],
   * whose DA frame is exactly [frameSize] bytes. The last block's extraData pads the frame.
   */
  fun conflation(
    startNumber: ULong,
    endNumber: ULong,
    frameSize: Int,
    firstTimestamp: Instant = baseTime + (startNumber.toLong() * 2).seconds,
    gasUsed: ULong = 1_000UL,
  ): BlocksConflation {
    fun timestamp(number: ULong) = firstTimestamp + ((number - startNumber).toLong() * 2).seconds
    val leading = (startNumber until endNumber).map {
      createBlock(number = it, timestamp = timestamp(it), gasUsed = gasUsed)
    }
    val leadingBytes = Int.SIZE_BYTES + leading.sumOf { encoder.encode(it).size }
    var extraDataSize = frameSize - leadingBytes
    repeat(10) {
      val last = createBlock(
        number = endNumber,
        timestamp = timestamp(endNumber),
        gasUsed = gasUsed,
        extraData = ByteArray(extraDataSize) { 0x2a },
      )
      val size = leadingBytes + encoder.encode(last).size
      if (size == frameSize) {
        return BlocksConflation(
          leading + last,
          ConflationCalculationResult(
            startNumber,
            endNumber,
            ConflationTrigger.BLOCKS_LIMIT,
            TracesCountersV5.EMPTY_TRACES_COUNT,
          ),
        )
      }
      extraDataSize += frameSize - size
    }
    error("could not build a frame of $frameSize bytes")
  }
}
