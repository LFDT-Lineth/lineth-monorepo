/*
 * Copyright Consensys Software Inc.
 *
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
}
