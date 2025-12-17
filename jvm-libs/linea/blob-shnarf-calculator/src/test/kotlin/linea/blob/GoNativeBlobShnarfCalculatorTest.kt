package linea.blob

import linea.kotlin.decodeHex
import linea.kotlin.encodeHex
import org.junit.jupiter.api.Assertions
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Nested
import org.junit.jupiter.api.Test
import kotlin.random.Random

class GoNativeBlobShnarfCalculatorTest {
  private lateinit var shnarfCalculator: GoNativeBlobShnarfCalculator

  @Nested
  inner class CompressorV0 {
    @BeforeEach
    fun beforeEach() {
      shnarfCalculator = GoNativeShnarfCalculatorFactory.getInstance(ShnarfCalculatorVersion.V1_2)
    }

    @Test
    fun `should calculate shnarf with eip4844 disabled`() {
      testCalculateShnarfEip4844Disabled(shnarfCalculator)
    }

    @Test
    fun `should calculate shnarf with eip4844 enabled`() {
      testCalculateShnarfEip4844Enabled(shnarfCalculator)
    }
  }

  @Nested
  inner class CompressorV1 {
    @BeforeEach
    fun beforeEach() {
      shnarfCalculator = GoNativeShnarfCalculatorFactory.getInstance(ShnarfCalculatorVersion.V1_2)
    }

    @Test
    fun `should calculate shnarf with eip4844 disabled`() {
      testCalculateShnarfEip4844Disabled(shnarfCalculator)
    }

    @Test
    fun `should calculate shnarf with eip4844 enabled`() {
      testCalculateShnarfEip4844Enabled(shnarfCalculator)
    }
  }

  fun testCalculateShnarfEip4844Disabled(calculator: GoNativeBlobShnarfCalculator) {
    testCalculate(calculator, eip4844Enabled = false) { result ->
      Assertions.assertNotNull(result.commitment)
      org.assertj.core.api.Assertions.assertThat(result.commitment.decodeHex()).hasSize(0)
      Assertions.assertNotNull(result.kzgProofContract)
      org.assertj.core.api.Assertions.assertThat(result.kzgProofContract.decodeHex()).hasSize(0)
      Assertions.assertNotNull(result.kzgProofSideCar)
      org.assertj.core.api.Assertions.assertThat(result.kzgProofSideCar.decodeHex()).hasSize(0)
    }
  }

  fun testCalculateShnarfEip4844Enabled(calculator: GoNativeBlobShnarfCalculator) {
    testCalculate(calculator, eip4844Enabled = true) { result ->
      Assertions.assertNotNull(result.commitment)
      org.assertj.core.api.Assertions.assertThat(result.commitment.decodeHex()).hasSize(48)
      Assertions.assertNotNull(result.kzgProofContract)
      org.assertj.core.api.Assertions.assertThat(result.kzgProofContract.decodeHex()).hasSize(48)
      Assertions.assertNotNull(result.kzgProofSideCar)
      org.assertj.core.api.Assertions.assertThat(result.kzgProofSideCar.decodeHex()).hasSize(48)
    }
  }

  private fun testCalculate(
    calculator: GoNativeBlobShnarfCalculator,
    eip4844Enabled: Boolean,
    assertResultFn: (CalculateShnarfResult) -> Unit = {},
  ) {
    val result = calculator.CalculateShnarf(
      eip4844Enabled = eip4844Enabled,
      compressedData = "ADAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDA=",
      parentStateRootHash = Random.Default.nextBytes(32).encodeHex(),
      finalStateRootHash = Random.Default.nextBytes(32).encodeHex(),
      prevShnarf = Random.Default.nextBytes(32).encodeHex(),
      conflationOrderStartingBlockNumber = 1L,
      conflationOrderUpperBoundariesLen = 3,
      conflationOrderUpperBoundaries = longArrayOf(10L, 20L, 30L),
    )

    Assertions.assertNotNull(result)
    org.assertj.core.api.Assertions.assertThat(result.errorMessage).isEmpty()

    // real response fields
    org.assertj.core.api.Assertions.assertThat(result.dataHash).isNotNull
    org.assertj.core.api.Assertions.assertThat(result.dataHash.decodeHex()).hasSize(32)
    org.assertj.core.api.Assertions.assertThat(result.snarkHash).isNotNull
    org.assertj.core.api.Assertions.assertThat(result.snarkHash.decodeHex()).hasSize(32)
    org.assertj.core.api.Assertions.assertThat(result.expectedX).isNotNull()
    org.assertj.core.api.Assertions.assertThat(result.expectedX.decodeHex()).hasSize(32)
    org.assertj.core.api.Assertions.assertThat(result.expectedY).isNotNull()
    org.assertj.core.api.Assertions.assertThat(result.expectedY.decodeHex()).hasSize(32)
    org.assertj.core.api.Assertions.assertThat(result.expectedShnarf).isNotNull
    org.assertj.core.api.Assertions.assertThat(result.expectedShnarf.decodeHex()).hasSize(32)
    assertResultFn(result)
  }

  // @Test
  // @Disabled("This test is meant to run locally to check for memory leaks")
  fun `shouldCalculateShnarf_checkMemory`() {
    val calculator = GoNativeShnarfCalculatorFactory.getInstance(ShnarfCalculatorVersion.V1_2)
    // if we enable forcedLeakBuffer, we will ger OOM after 4-6 iterations
    // Exception in thread "JNA Cleaner" java.lang.OutOfMemoryError: Java heap space
    // val forcedLeakBuffer = mutableListOf<CalculateShnarfResult>()
    for (j in 0..100) {
      for (i in 0..100_000) {
        calculator.CalculateShnarf(
          eip4844Enabled = false,
          compressedData = "ADAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDA=",
          parentStateRootHash = Random.Default.nextBytes(32).encodeHex(),
          finalStateRootHash = Random.Default.nextBytes(32).encodeHex(),
          prevShnarf = Random.Default.nextBytes(32).encodeHex(),
          conflationOrderStartingBlockNumber = 0L,
          conflationOrderUpperBoundariesLen = 2,
          conflationOrderUpperBoundaries = longArrayOf(10L, 20L),
        )
        // .let { result -> forcedLeakBuffer.add(result) }
      }
      System.gc()
      Thread.sleep(1000)
      println(
        "total=${Runtime.getRuntime().totalMemory()}, " +
          "free=${Runtime.getRuntime().freeMemory()}, " +
          "max=${Runtime.getRuntime().maxMemory()}",
      )
    }
  }
}
