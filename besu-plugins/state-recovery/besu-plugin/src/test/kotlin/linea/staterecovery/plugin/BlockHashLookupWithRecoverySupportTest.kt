package linea.staterecovery.plugin

import linea.kotlin.decodeHex
import linea.kotlin.toHexStringUInt256
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.hyperledger.besu.datatypes.Hash
import org.junit.jupiter.api.Test

class BlockHashLookupWithRecoverySupportTest {

  @Test
  fun `addLookbackHashes should validate items are sequential`() {
    val lookback = BlockHashLookupWithRecoverySupport(lookbackWindow = 3UL)
    lookback.addLookbackHashes(
      mapOf(
        1UL to hashOf(1UL),
        2UL to hashOf(3UL),
      ),
    )

    assertThatThrownBy {
      lookback.addLookbackHashes(
        mapOf(
          1UL to hashOf(1UL),
          3UL to hashOf(3UL),
        ),
      )
    }
      .isInstanceOf(IllegalArgumentException::class.java)
      .hasMessageContaining("must be sequential")
  }

  @Test
  fun `addHeadBlockHash should update and prune the lookback hashes outside the lookback window`() {
    val lookback = BlockHashLookupWithRecoverySupport(
      lookbackWindow = 3UL,
    )

    lookback.addHeadBlockHash(0UL, hashOf(123UL))
    lookback.addHeadBlockHash(1UL, hashOf(1UL))
    lookback.addHeadBlockHash(2UL, hashOf(2UL))
    assertThat(lookback.getHash(0)).isEqualTo(hashOfBesuType(123UL))
    assertThat(lookback.getHash(1)).isEqualTo(hashOfBesuType(1UL))
    assertThat(lookback.getHash(2)).isEqualTo(hashOfBesuType(2UL))

    lookback.addHeadBlockHash(3UL, hashOf(3UL))
    lookback.addHeadBlockHash(4UL, hashOf(4UL))
    lookback.addHeadBlockHash(5UL, hashOf(5UL))
    assertThat(lookback.getHash(0)).isEqualTo(Hash.ZERO)
    assertThat(lookback.getHash(1)).isEqualTo(Hash.ZERO)
    assertThat(lookback.getHash(2)).isEqualTo(hashOfBesuType(2UL))
    assertThat(lookback.getHash(3)).isEqualTo(hashOfBesuType(3UL))
    assertThat(lookback.getHash(4)).isEqualTo(hashOfBesuType(4UL))
    assertThat(lookback.getHash(5)).isEqualTo(hashOfBesuType(5UL))

    lookback.addHeadBlockHash(6UL, hashOf(6UL))
    assertThat(lookback.getHash(2)).isEqualTo(Hash.ZERO)
  }

  private fun hashOf(number: ULong): ByteArray = number.toHexStringUInt256().decodeHex()
  private fun hashOfBesuType(number: ULong): Hash = Hash.fromHexString(number.toHexStringUInt256())
}
