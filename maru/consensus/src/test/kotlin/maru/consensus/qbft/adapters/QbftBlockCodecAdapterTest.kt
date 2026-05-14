/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus.qbft.adapters

import maru.core.ext.DataGenerators
import org.assertj.core.api.Assertions.assertThat
import org.hyperledger.besu.ethereum.rlp.RLP
import org.junit.jupiter.api.Test

class QbftBlockCodecAdapterTest {
  @Test
  fun `can encode and decode same value`() {
    val beaconBlock = DataGenerators.randomBeaconBlock(10U)
    val testValue = QbftBlockAdapter(beaconBlock)
    val qbftBlockCodecAdapter = QbftBlockCodecAdapter

    val encodedData = RLP.encode { rlpOutput -> qbftBlockCodecAdapter.writeTo(testValue, rlpOutput) }
    val decodedValue = qbftBlockCodecAdapter.readFrom(RLP.input(encodedData))
    assertThat(decodedValue).isEqualTo(testValue)
  }
}
