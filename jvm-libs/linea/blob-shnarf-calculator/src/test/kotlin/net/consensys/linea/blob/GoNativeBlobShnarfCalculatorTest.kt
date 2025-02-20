package net.consensys.linea.blob

import linea.kotlin.decodeHex
import linea.kotlin.encodeHex
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Assertions.assertNotNull
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
      shnarfCalculator = GoNativeShnarfCalculatorFactory.getInstance(ShnarfCalculatorVersion.V0_1_0)
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
      shnarfCalculator = GoNativeShnarfCalculatorFactory.getInstance(ShnarfCalculatorVersion.V1_0_1)
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

  fun testCalculateShnarfEip4844Disabled(
    calculator: GoNativeBlobShnarfCalculator
  ) {
    testCalculate(calculator, eip4844Enabled = false) { result ->
      assertNotNull(result.commitment)
      assertThat(result.commitment.decodeHex()).hasSize(0)
      assertNotNull(result.kzgProofContract)
      assertThat(result.kzgProofContract.decodeHex()).hasSize(0)
      assertNotNull(result.kzgProofSideCar)
      assertThat(result.kzgProofSideCar.decodeHex()).hasSize(0)
    }
  }

  fun testCalculateShnarfEip4844Enabled(
    calculator: GoNativeBlobShnarfCalculator
  ) {
    testCalculate(calculator, eip4844Enabled = true) { result ->
      assertNotNull(result.commitment)
      assertThat(result.commitment.decodeHex()).hasSize(48)
      assertNotNull(result.kzgProofContract)
      assertThat(result.kzgProofContract.decodeHex()).hasSize(48)
      assertNotNull(result.kzgProofSideCar)
      assertThat(result.kzgProofSideCar.decodeHex()).hasSize(48)
    }
  }

  private fun testCalculate(
    calculator: GoNativeBlobShnarfCalculator,
    eip4844Enabled: Boolean,
    assertResultFn: (CalculateShnarfResult) -> Unit = {}
  ) {
    val result = calculator.CalculateShnarf(
      eip4844Enabled = eip4844Enabled,
      compressedData = "ADAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDA=",
      parentStateRootHash = Random.nextBytes(32).encodeHex(),
      finalStateRootHash = Random.nextBytes(32).encodeHex(),
      prevShnarf = Random.nextBytes(32).encodeHex(),
      conflationOrderStartingBlockNumber = 1L,
      conflationOrderUpperBoundariesLen = 3,
      conflationOrderUpperBoundaries = longArrayOf(10L, 20L, 30L)
    )

    assertNotNull(result)
    assertThat(result.errorMessage).isEmpty()

    // real response fields
    assertThat(result.dataHash).isNotNull
    assertThat(result.dataHash.decodeHex()).hasSize(32)
    assertThat(result.snarkHash).isNotNull
    assertThat(result.snarkHash.decodeHex()).hasSize(32)
    assertThat(result.expectedX).isNotNull()
    assertThat(result.expectedX.decodeHex()).hasSize(32)
    assertThat(result.expectedY).isNotNull()
    assertThat(result.expectedY.decodeHex()).hasSize(32)
    assertThat(result.expectedShnarf).isNotNull
    assertThat(result.expectedShnarf.decodeHex()).hasSize(32)
    assertResultFn(result)
  }

  // @Test
  // @Disabled("This test is meant to run locally to check for memory leaks")
  fun `shouldCalculateShnarf_checkMemory`() {
    val calculator = GoNativeShnarfCalculatorFactory.getInstance(ShnarfCalculatorVersion.V1_0_1)
    // if we enable forcedLeakBuffer, we will ger OOM after 4-6 iterations
    // Exception in thread "JNA Cleaner" java.lang.OutOfMemoryError: Java heap space
    // val forcedLeakBuffer = mutableListOf<CalculateShnarfResult>()
    for (j in 0..100) {
      for (i in 0..100_000) {
        calculator.CalculateShnarf(
          eip4844Enabled = false,
          compressedData = "ADAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDA=",
          parentStateRootHash = Random.nextBytes(32).encodeHex(),
          finalStateRootHash = Random.nextBytes(32).encodeHex(),
          prevShnarf = Random.nextBytes(32).encodeHex(),
          conflationOrderStartingBlockNumber = 0L,
          conflationOrderUpperBoundariesLen = 2,
          conflationOrderUpperBoundaries = longArrayOf(10L, 20L)
        )
        // .let { result -> forcedLeakBuffer.add(result) }
      }
      System.gc()
      Thread.sleep(1000)
      println(
        "total=${Runtime.getRuntime().totalMemory()}, " +
          "free=${Runtime.getRuntime().freeMemory()}, " +
          "max=${Runtime.getRuntime().maxMemory()}"
      )
    }
  }
}
