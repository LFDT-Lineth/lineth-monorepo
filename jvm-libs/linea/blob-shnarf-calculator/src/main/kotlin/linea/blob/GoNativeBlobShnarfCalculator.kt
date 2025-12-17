package linea.blob

import com.sun.jna.Library
import com.sun.jna.Native
import com.sun.jna.Structure

// Equivalent to the C response struct used in CGO
class CalculateShnarfResult(
  @JvmField var commitment: String,
  @JvmField var kzgProofContract: String,
  @JvmField var kzgProofSideCar: String,
  @JvmField var dataHash: String,
  @JvmField var snarkHash: String,
  @JvmField var expectedX: String,
  @JvmField var expectedY: String,
  @JvmField var expectedShnarf: String,
  // error message is empty if there is no error
  // it cannot be null because JNA call blow up with segfault.
  @JvmField var errorMessage: String,
) : Structure() {

  // JNA requires a default constructor
  constructor() : this("", "", "", "", "", "", "", "", "")

  override fun getFieldOrder(): List<String> {
    return listOf(
      "commitment",
      "kzgProofContract",
      "kzgProofSideCar",
      "dataHash",
      "snarkHash",
      "expectedX",
      "expectedY",
      "expectedShnarf",
      "errorMessage",
    )
  }
}

interface GoNativeBlobShnarfCalculator {
  // Equivalent to the Go CalculateShnarf function acting as bridge
  // between C and Go code.
  // It prepares a Response object by computing all the fields except for the
  // proof.
  fun CalculateShnarf(
    eip4844Enabled: Boolean,
    compressedData: String,
    parentStateRootHash: String,
    finalStateRootHash: String,
    prevShnarf: String,
    conflationOrderStartingBlockNumber: Long,
    conflationOrderUpperBoundariesLen: Int,
    conflationOrderUpperBoundaries: LongArray,
  ): CalculateShnarfResult
}

// Just to not expose JNA to the clients
internal interface GoNativeBlobShnarfCalculatorJna : GoNativeBlobShnarfCalculator, Library

enum class ShnarfCalculatorVersion(val version: String) {
  V1_2("v1.2.0"),
}

class GoNativeShnarfCalculatorFactory {
  companion object {
    private fun getLibFileName(version: String) = "shnarf_calculator_jna_$version"

    fun getInstance(version: ShnarfCalculatorVersion): GoNativeBlobShnarfCalculator {
      val extractedLibFile = Native.extractFromResourcePath(
        getLibFileName(version.version),
        GoNativeShnarfCalculatorFactory::class.java.classLoader,
      )
      return Native.load(
        extractedLibFile.toString(),
        GoNativeBlobShnarfCalculatorJna::class.java,
      )
    }
  }
}
