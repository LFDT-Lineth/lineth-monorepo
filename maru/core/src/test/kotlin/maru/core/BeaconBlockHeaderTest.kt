/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.core

import maru.core.ext.DataGenerators
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import org.mockito.Mockito
import org.mockito.kotlin.times
import org.mockito.kotlin.verify
import org.mockito.kotlin.whenever
import kotlin.random.Random

class BeaconBlockHeaderTest {
  @Test
  fun `hash is not initialised on header construction`() {
    val beaconBlockIdHashFunction = Mockito.mock(BeaconBlockIdHashFunction::class.java)

    val header =
      DataGenerators
        .randomBeaconBlockHeader(1u)
        .copy(beaconBlockIdHashFunction = beaconBlockIdHashFunction)
    verify(beaconBlockIdHashFunction, Mockito.never()).invoke(header)
  }

  @Test
  fun `hash is calculated only once`() {
    val beaconBlockIdHashFunction = Mockito.mock(BeaconBlockIdHashFunction::class.java)
    val header =
      DataGenerators
        .randomBeaconBlockHeader(1u)
        .copy(beaconBlockIdHashFunction = beaconBlockIdHashFunction)
    whenever(beaconBlockIdHashFunction.invoke(header)).thenReturn(Random.nextBytes(32))

    verify(beaconBlockIdHashFunction, Mockito.never()).invoke(header)

    val hash1 = header.beaconBlockIdHash()
    val hash2 = header.beaconBlockIdHash()
    assertThat(hash1).isEqualTo(hash2)
    verify(beaconBlockIdHashFunction, times(1)).invoke(header)
  }
}
