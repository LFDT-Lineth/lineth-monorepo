package net.consensys.zkevm.coordination.blob

import io.micrometer.core.instrument.simple.SimpleMeterRegistry
import linea.domain.BlockIntervals
import linea.kotlin.decodeHex
import linea.kotlin.encodeHex
import net.consensys.linea.blob.CalculateShnarfResult
import net.consensys.linea.blob.GoNativeBlobShnarfCalculator
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.linea.metrics.micrometer.MicrometerMetricsFacade
import net.consensys.zkevm.ethereum.coordination.blob.GoBackedBlobShnarfCalculator
import net.consensys.zkevm.ethereum.coordination.blob.ShnarfResult
import org.apache.tuweni.bytes.Bytes32
import org.apache.tuweni.bytes.Bytes48
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.assertThrows
import org.mockito.Mockito.mock
import org.mockito.kotlin.any
import org.mockito.kotlin.eq
import org.mockito.kotlin.verify
import org.mockito.kotlin.whenever

class GoBackedCalculateShnarfCalculatorTest {
  private lateinit var delegate: GoNativeBlobShnarfCalculator
  private lateinit var calculator: GoBackedBlobShnarfCalculator
  private val meterRegistry = SimpleMeterRegistry()
  private val metricsFacade: MetricsFacade = MicrometerMetricsFacade(registry = meterRegistry, "linea")
  private val compressedData = byteArrayOf(0b01001101, 0b01100001, 0b01101110)
  private val compressedDataBase64String = "TWFu"
  private val parentStateRootHash = Bytes32.random().toArray()
  private val finalStateRootHash = Bytes32.random().toArray()
  private val prevShnarf = Bytes32.random().toArray()
  private val fakeCalculationResult = CalculateShnarfResult(
    commitment = Bytes48.random().toArray().encodeHex(),
    kzgProofContract = Bytes48.random().toArray().encodeHex(),
    kzgProofSideCar = Bytes48.random().toArray().encodeHex(),
    dataHash = Bytes32.random().toArray().encodeHex(),
    snarkHash = Bytes32.random().toArray().encodeHex(),
    expectedX = Bytes32.random().toArray().encodeHex(),
    expectedY = Bytes32.random().toArray().encodeHex(),
    expectedShnarf = Bytes32.random().toArray().encodeHex(),
    errorMessage = ""
  )
  private val expectedShnarfParsedResult = ShnarfResult(
    dataHash = fakeCalculationResult.dataHash.decodeHex(),
    snarkHash = fakeCalculationResult.snarkHash.decodeHex(),
    expectedX = fakeCalculationResult.expectedX.decodeHex(),
    expectedY = fakeCalculationResult.expectedY.decodeHex(),
    expectedShnarf = fakeCalculationResult.expectedShnarf.decodeHex(),
    commitment = fakeCalculationResult.commitment.decodeHex(),
    kzgProofSideCar = fakeCalculationResult.kzgProofSideCar.decodeHex(),
    kzgProofContract = fakeCalculationResult.kzgProofContract.decodeHex()
  )

  @BeforeEach
  fun beforeEach() {
    delegate = mock()
    calculator = GoBackedBlobShnarfCalculator(delegate, metricsFacade)
  }

  @Test
  fun `when delegate calculator returns response, should return ShnarfResult`() {
    whenever(delegate.CalculateShnarf(eq(true), any(), any(), any(), any(), any(), any(), any()))
      .thenReturn(fakeCalculationResult, null)

    val result = calculator.calculateShnarf(
      compressedData = compressedData,
      parentStateRootHash = parentStateRootHash,
      finalStateRootHash = finalStateRootHash,
      prevShnarf = prevShnarf,
      conflationOrder = BlockIntervals(1UL, listOf(5UL, 10UL))
    )

    verify(delegate).CalculateShnarf(
      eq(true),
      eq(compressedDataBase64String),
      eq(parentStateRootHash.encodeHex()),
      eq(finalStateRootHash.encodeHex()),
      eq(prevShnarf.encodeHex()),
      eq(1L),
      eq(2),
      eq(longArrayOf(5L, 10L))
    )
    assertThat(result).isEqualTo(expectedShnarfParsedResult)
  }

  @Test
  fun `should work with block number up to 2^63`() {
    whenever(delegate.CalculateShnarf(eq(true), any(), any(), any(), any(), any(), any(), any()))
      .thenReturn(fakeCalculationResult, null)

    val result = calculator.calculateShnarf(
      compressedData = compressedData,
      parentStateRootHash = parentStateRootHash,
      finalStateRootHash = finalStateRootHash,
      prevShnarf = prevShnarf,
      conflationOrder = BlockIntervals(Long.MAX_VALUE.toULong(), listOf(5UL, Long.MAX_VALUE.toULong()))
    )

    verify(delegate).CalculateShnarf(
      eq(true),
      eq(compressedDataBase64String),
      eq(parentStateRootHash.encodeHex()),
      eq(finalStateRootHash.encodeHex()),
      eq(prevShnarf.encodeHex()),
      eq(Long.MAX_VALUE),
      eq(2),
      eq(longArrayOf(5L, Long.MAX_VALUE))
    )

    assertThat(result).isEqualTo(expectedShnarfParsedResult)
  }

  @Test
  fun `when delegate calculator returns error, should throw RuntimeException`() {
    val errorResult = CalculateShnarfResult().apply { errorMessage = "forced test error" }
    whenever(delegate.CalculateShnarf(eq(true), any(), any(), any(), any(), any(), any(), any()))
      .thenReturn(errorResult)

    assertThrows<RuntimeException> {
      calculator.calculateShnarf(
        compressedData = ByteArray(0),
        parentStateRootHash = ByteArray(0),
        finalStateRootHash = ByteArray(0),
        prevShnarf = ByteArray(0),
        conflationOrder = BlockIntervals(0UL, listOf(0UL))
      )
    }
  }
}
