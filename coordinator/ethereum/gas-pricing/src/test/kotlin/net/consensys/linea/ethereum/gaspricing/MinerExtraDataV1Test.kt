package net.consensys.linea.ethereum.gaspricing

import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.assertThrows
import org.junit.jupiter.params.ParameterizedTest
import org.junit.jupiter.params.provider.Arguments
import org.junit.jupiter.params.provider.MethodSource
import java.util.stream.Stream

class MinerExtraDataV1Test {
  @ParameterizedTest
  @MethodSource("minerExtraDataPositiveTestCases")
  fun encode(encodedMinerExtraData: String, minerExtraData: MinerExtraDataV1) {
    assertThat(encodedMinerExtraData).hasSize(66)
    assertThat(minerExtraData.encode().lowercase()).isEqualTo(encodedMinerExtraData.lowercase())
  }

  @ParameterizedTest
  @MethodSource("minerExtraDataPositiveTestCases")
  fun decode(encodedMinerExtraData: String, minerExtraData: MinerExtraDataV1) {
    assertThat(encodedMinerExtraData).hasSize(66)
    val decodedMinerExtraData = MinerExtraDataV1.decodeV1(encodedMinerExtraData)
    assertThat(decodedMinerExtraData).isEqualTo(minerExtraData)
  }

  @Test
  fun `decoding unsupported version fails`() {
    val encodedMinerExtraDataWithUnsupportedVersion =
      readableBytesToPaddedHex("0x02|${0x7FFF_FFFE}|${0x7FFF_FFFF}|${0x8000_0000}")

    assertThrows<IllegalArgumentException> {
      MinerExtraDataV1.decodeV1(encodedMinerExtraDataWithUnsupportedVersion)
    }
  }

  companion object {
    private val paddingLength = run {
      val minerExtraDataExpectedSize = Byte.SIZE_BYTES + 3 * Int.SIZE_BYTES
      32 - minerExtraDataExpectedSize
    }

    private fun readableBytesToPaddedHex(readableBytes: String): String {
      return readableBytes.replace(Regex("[|_]"), "") + "00".repeat(paddingLength)
    }

    @JvmStatic
    fun minerExtraDataPositiveTestCases(): Stream<Arguments> {
      val minerExtraData1 = MinerExtraDataV1(
        fixedCostInKWei = Int.MAX_VALUE.toUInt() - 1u, // 2147483646u, 2147483646, 0x7FFF_FFFE
        variableCostInKWei = Int.MAX_VALUE.toUInt(), // 2147483647u, 2147483647, 0x7FFF_FFFF
        ethGasPriceInKWei = Int.MAX_VALUE.toUInt() + 1u, // 2147483648u, -2147483648, 0x8000_0000
      )
      val encodedMinerExtraData1 = readableBytesToPaddedHex("0x01|7FFF_FFFE|7FFF_FFFF|8000_0000")

      val minerExtraData2 = MinerExtraDataV1(
        fixedCostInKWei = UInt.MIN_VALUE, // 0u, 0, 0x0000_0000
        variableCostInKWei = Int.MAX_VALUE.toUInt(), // 2147483647u, 2147483647, 0x7FFF_FFFF
        ethGasPriceInKWei = UInt.MAX_VALUE, // 4294967295u, -1, 0xFFFF_FFFF
      )
      val encodedMinerExtraData2 = readableBytesToPaddedHex("0x01|0000_0000|7FFF_FFFF|FFFF_FFFF")

      return Stream.of(
        Arguments.of(encodedMinerExtraData1, minerExtraData1),
        Arguments.of(encodedMinerExtraData2, minerExtraData2),
      )
    }
  }
}
