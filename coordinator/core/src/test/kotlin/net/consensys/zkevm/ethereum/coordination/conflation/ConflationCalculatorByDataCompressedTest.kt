package net.consensys.zkevm.ethereum.coordination.conflation

import kotlinx.datetime.Instant
import net.consensys.linea.traces.fakeTracesCountersV2
import net.consensys.zkevm.domain.BlockCounters
import net.consensys.zkevm.domain.ConflationTrigger
import net.consensys.zkevm.ethereum.coordination.blob.BlobCompressionException
import net.consensys.zkevm.ethereum.coordination.blob.BlobCompressor
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.mockito.kotlin.any
import org.mockito.kotlin.eq
import org.mockito.kotlin.mock
import org.mockito.kotlin.whenever
import kotlin.random.Random

class ConflationCalculatorByDataCompressedTest {
  private lateinit var calculator: ConflationCalculatorByDataCompressed
  private lateinit var blobCompressor: BlobCompressor

  @BeforeEach
  fun beforeEach() {
    blobCompressor =
      mock {
        on { canAppendBlock(any<ByteArray>()) }.thenReturn(true)
        on { appendBlock(any<ByteArray>()) }.thenAnswer { invocation ->
          val block = invocation.arguments[0] as ByteArray
          BlobCompressor.AppendResult(
            blockAppended = true,
            compressedSizeBefore = 0,
            compressedSizeAfter = block.size,
          )
        }
      }
    calculator =
      ConflationCalculatorByDataCompressed(
        blobCompressor,
      )
  }

  @Test
  fun `checkOverflow should return null when blobCompressor can append block`() {
    whenever(blobCompressor.canAppendBlock(any<ByteArray>())).thenReturn(true)
    assertThat(calculator.checkOverflow(blockCounters())).isNull()

    whenever(blobCompressor.canAppendBlock(any<ByteArray>())).thenReturn(false)
    assertThat(calculator.checkOverflow(blockCounters())).isEqualTo(
      ConflationCalculator.OverflowTrigger(
        trigger = ConflationTrigger.DATA_LIMIT,
        singleBlockOverSized = true,
      ),
    )
  }

  @Test
  fun `checkOverflow should throw error when compressor returns error`() {
    whenever(blobCompressor.canAppendBlock(any<ByteArray>()))
      .thenReturn(true)
    assertThat(calculator.checkOverflow(blockCounters())).isNull()

    whenever(blobCompressor.canAppendBlock(any<ByteArray>())).thenReturn(false)
    assertThat(calculator.checkOverflow(blockCounters())).isEqualTo(
      ConflationCalculator.OverflowTrigger(
        trigger = ConflationTrigger.DATA_LIMIT,
        singleBlockOverSized = true,
      ),
    )
  }

  private fun ConflationCalculatorByDataCompressed.checkAndAppendBlock(blockCounters: BlockCounters) {
    val trigger = this.checkOverflow(blockCounters)
    assertThat(trigger).isNull()
    this.appendBlock(blockCounters)
  }

  @Test
  fun `appendBlock should accumulate data`() {
    val block1RawData = Random.nextBytes(100)
    val block2RawData = Random.nextBytes(200)
    val block3RawData = Random.nextBytes(300)
    whenever(blobCompressor.appendBlock(eq(block1RawData))).thenReturn(
      BlobCompressor.AppendResult(
        blockAppended = true,
        compressedSizeBefore = 0,
        compressedSizeAfter = 50,
      ),
    )
    whenever(blobCompressor.appendBlock(eq(block2RawData))).thenReturn(
      BlobCompressor.AppendResult(
        blockAppended = true,
        compressedSizeBefore = 2,
        compressedSizeAfter = 100,
      ),
    )
    whenever(blobCompressor.appendBlock(eq(block3RawData))).thenReturn(
      BlobCompressor.AppendResult(
        blockAppended = true,
        compressedSizeBefore = 3,
        compressedSizeAfter = 150,
      ),
    )

    calculator.checkAndAppendBlock(blockCounters(block1RawData))
    calculator.checkAndAppendBlock(blockCounters(block2RawData))
    calculator.startNewBatch()
    calculator.checkAndAppendBlock(blockCounters(block3RawData))

    assertThat(calculator.dataSizeUpToLastBatch).isEqualTo(100u)
    assertThat(calculator.dataSize).isEqualTo(150u)

    calculator.startNewBatch()
    assertThat(calculator.dataSizeUpToLastBatch).isEqualTo(150u)
    assertThat(calculator.dataSize).isEqualTo(150u)
  }

  @Test
  fun `appendBlock should throw error when append cannot append anymore`() {
    val block1RawData = Random.nextBytes(100)
    whenever(blobCompressor.appendBlock(eq(block1RawData))).thenReturn(
      BlobCompressor.AppendResult(
        blockAppended = false,
        compressedSizeBefore = 0,
        compressedSizeAfter = 5000,
      ),
    )

    calculator.checkOverflow(blockCounters(block1RawData))
    assertThatThrownBy { calculator.appendBlock(blockCounters(block1RawData)) }
      .isInstanceOf(IllegalStateException::class.java)
      .hasMessage("Trying to append a block that does not fit in the blob.")
  }

  @Test
  fun `appendBlock should throw error when compression throws error`() {
    val block1RawData = Random.nextBytes(100)
    whenever(blobCompressor.appendBlock(eq(block1RawData)))
      .thenThrow(BlobCompressionException("Invalid RLP encoding."))

    calculator.checkOverflow(blockCounters(block1RawData))
    assertThatThrownBy { calculator.appendBlock(blockCounters(block1RawData)) }
      .isInstanceOf(BlobCompressionException::class.java)
      .hasMessage("Invalid RLP encoding.")
  }

  @Test
  fun `reset should reset only after blob has been filled and data retrieved`() {
    val block1RawData = Random.nextBytes(100)
    val block2RawData = Random.nextBytes(200)
    val block3RawData = Random.nextBytes(300)
    whenever(blobCompressor.appendBlock(eq(block1RawData))).thenReturn(
      BlobCompressor.AppendResult(
        blockAppended = true,
        compressedSizeBefore = 0,
        compressedSizeAfter = 50,
      ),
    )
    whenever(blobCompressor.appendBlock(eq(block2RawData))).thenReturn(
      BlobCompressor.AppendResult(
        blockAppended = true,
        compressedSizeBefore = 2,
        compressedSizeAfter = 100,
      ),
    )
    whenever(blobCompressor.canAppendBlock(eq(block3RawData)))
      .thenReturn(false)
      .thenReturn(true)
    whenever(blobCompressor.appendBlock(eq(block3RawData))).thenReturn(
      BlobCompressor.AppendResult(
        blockAppended = true,
        compressedSizeBefore = 3,
        compressedSizeAfter = 150,
      ),
    )
    calculator.checkAndAppendBlock(blockCounters(block1RawData))
    calculator.startNewBatch()
    calculator.reset()
    assertThat(calculator.dataSizeUpToLastBatch).isEqualTo(50u)
    assertThat(calculator.dataSize).isEqualTo(50u)

    calculator.checkAndAppendBlock(blockCounters(block2RawData))
    assertThat(calculator.dataSizeUpToLastBatch).isEqualTo(50u)
    assertThat(calculator.dataSize).isEqualTo(100u)

    calculator.checkOverflow(blockCounters(block3RawData))
    calculator.startNewBatch()
    calculator.getCompressedData()
    calculator.reset()
    assertThat(calculator.dataSizeUpToLastBatch).isEqualTo(0u)
    assertThat(calculator.dataSize).isEqualTo(0u)

    calculator.checkAndAppendBlock(blockCounters(block3RawData))
    assertThat(calculator.dataSizeUpToLastBatch).isEqualTo(0u)
    assertThat(calculator.dataSize).isEqualTo(150u)
  }

  private fun blockCounters(rlpRawData: ByteArray = ByteArray(1)): BlockCounters = BlockCounters(
    blockNumber = 0u,
    blockTimestamp = Instant.parse("2021-01-01T00:00:00Z"),
    tracesCounters = fakeTracesCountersV2(0u),
    blockRLPEncoded = rlpRawData,
  )
}
