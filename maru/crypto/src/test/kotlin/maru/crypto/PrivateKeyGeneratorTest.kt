/*
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.crypto

import linea.kotlin.decodeHex
import maru.crypto.PrivateKeyGenerator.generatePrivateKey
import maru.crypto.PrivateKeyGenerator.getKeyData
import maru.crypto.PrivateKeyGenerator.getKeyDataByPrefixedKey
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.RepeatedTest
import org.junit.jupiter.api.Test

class PrivateKeyGeneratorTest {
  @RepeatedTest(100)
  fun `should generator private key`() {
    val keyData = generatePrivateKey()
    val recoveredDataFromPrivKey = getKeyData(privateKey = keyData.privateKey)
    val recoveredDataFromPrivKeyPrefixed = getKeyDataByPrefixedKey(prefixedPrivateKey = keyData.prefixedPrivateKey)
    assertThat(keyData).isEqualTo(recoveredDataFromPrivKey)
    assertThat(keyData).isEqualTo(recoveredDataFromPrivKeyPrefixed)
  }

  @RepeatedTest(100)
  fun `should yield correct key data`() {
    val prefixedPrivKey = "0x08021220289909347c7865907cabb5b0ed59f967ef31717b5cb01beee279f5aa73fe48a9".decodeHex()
    val privKey = "0x289909347c7865907cabb5b0ed59f967ef31717b5cb01beee279f5aa73fe48a9".decodeHex()
    val address = "0xa275d33b6d691cf5212850ff2d44643f02c30d37".decodeHex()

    val keyInfo = getKeyData(privKey)
    assertThat(keyInfo.prefixedPrivateKey).isEqualTo(prefixedPrivKey)
    assertThat(keyInfo.privateKey).isEqualTo(privKey)
    assertThat(keyInfo.address).isEqualTo(address)
  }

  @Test
  fun `short keys with leading zero bytes keep a canonical 32-byte prefixed payload`() {
    // Regression for the torn-KeyData flake: BigInteger round-trips drop leading zero bytes, so a
    // 31-byte key used to produce a protobuf payload with length byte 0x1F, and takeLast(32) readers
    // recovered a corrupted key (length byte + 31 key bytes).
    val shortKey = byteArrayOf(0x00) + ByteArray(31) { (it + 1).toByte() }
    val keyData = getKeyData(shortKey)

    assertThat(keyData.prefixedPrivateKey).hasSize(36)
    assertThat(keyData.prefixedPrivateKey.take(4))
      .containsExactly(0x08.toByte(), 0x02.toByte(), 0x12.toByte(), 0x20.toByte())
    assertThat(keyData.prefixedPrivateKey.takeLast(32).toByteArray())
      .isEqualTo(byteArrayOf(0x00) + ByteArray(31) { (it + 1).toByte() })
    assertThat(getKeyDataByPrefixedKey(keyData.prefixedPrivateKey)).isEqualTo(keyData)
  }
}
