package lineth.conflation.calculators

import linea.domain.BlocksConflation
import linea.domain.ConflationCalculationResult
import linea.domain.ConflationTrigger
import linea.domain.TransactionFactory
import linea.domain.createBlock
import linea.domain.toBesu
import linea.rlp.RLP
import lineth.coordination.riscv.da.FakeDaStream
import lineth.da.DaConstants.SEGMENT_LENGTH_PREFIX_BYTES
import lineth.encoding.BlockEncoder
import net.consensys.linea.traces.TracesCountersV5
import kotlin.time.Duration
import kotlin.time.Duration.Companion.seconds
import kotlin.time.Instant

/** Conflations with an exact DA segment size, for tests of the RISC-V blob and rollup calculators on [FakeDaStream]. */
object DaTestConflations {
  val encoder = BlockEncoder { RLP.encodeBlock(it.toBesu()) }
  val baseTime: Instant = Instant.parse("2026-10-01T00:00:00Z")
  val genesisDataRollingHash = ByteArray(32) { 0x11 }

  /** One-block conflation whose DA segment is exactly [segmentSize] bytes. */
  fun conflation(
    number: ULong,
    segmentSize: Int,
    timestamp: Instant = baseTime + (number.toLong() * 2).seconds,
    gasUsed: ULong = 1_000UL,
  ): BlocksConflation = conflation(number, number, segmentSize, timestamp, gasUsed)

  /**
   * Conflation of blocks [startNumber]..[endNumber], [blockInterval] apart from [firstTimestamp], each using [gasUsed]
   * and holding [txsPerBlock] signed transactions, whose DA segment is exactly [segmentSize] bytes. The last block's
   * extraData pads the segment.
   */
  fun conflation(
    startNumber: ULong,
    endNumber: ULong,
    segmentSize: Int,
    firstTimestamp: Instant = baseTime + (startNumber.toLong() * 2).seconds,
    gasUsed: ULong = 1_000UL,
    blockInterval: Duration = 2.seconds,
    txsPerBlock: Int = 0,
  ): BlocksConflation {
    fun timestamp(number: ULong) = firstTimestamp + blockInterval * (number - startNumber).toInt()
    fun transactions() = List(txsPerBlock) { TransactionFactory.createTransaction(nonce = it.toULong(), chainId = 1UL) }
    val leading = (startNumber until endNumber).map {
      createBlock(number = it, timestamp = timestamp(it), gasUsed = gasUsed, transactions = transactions())
    }
    val lastTransactions = transactions()
    val leadingBytes = SEGMENT_LENGTH_PREFIX_BYTES + leading.sumOf { encoder.encode(it).size }
    var extraDataSize = segmentSize - leadingBytes
    repeat(10) {
      val last = createBlock(
        number = endNumber,
        timestamp = timestamp(endNumber),
        gasUsed = gasUsed,
        transactions = lastTransactions,
        extraData = ByteArray(extraDataSize) { 0x2a },
      )
      val size = leadingBytes + encoder.encode(last).size
      if (size == segmentSize) {
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
      extraDataSize += segmentSize - size
    }
    error("could not build a segment of $segmentSize bytes")
  }
}
