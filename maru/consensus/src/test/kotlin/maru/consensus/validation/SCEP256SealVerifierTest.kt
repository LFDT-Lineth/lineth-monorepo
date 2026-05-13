/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus.validation

import com.github.michaelbull.result.Ok
import maru.core.Seal
import maru.core.Validator
import maru.core.ext.DataGenerators
import maru.crypto.SecpCrypto
import org.apache.tuweni.bytes.Bytes32
import org.assertj.core.api.Assertions.assertThat
import org.hyperledger.besu.ethereum.core.Util
import org.junit.jupiter.api.Test

class SCEP256SealVerifierTest {
  private val signatureAlgorithm = SecpCrypto.signatureAlgorithm
  private val verifier = SCEP256SealVerifier()
  private val keypair = signatureAlgorithm.generateKeyPair()
  private val validator = Validator(Util.publicKeyToAddress(keypair.publicKey).bytes.toArray())

  @Test
  fun `test extract validator`() {
    val beaconBlockHeader = DataGenerators.randomBeaconBlockHeader(101u)
    val signature = signatureAlgorithm.sign(Bytes32.wrap(beaconBlockHeader.hash), keypair)
    val seal = Seal(signature.encodedBytes().toArray())
    val result = verifier.extractValidator(seal, beaconBlockHeader)
    assertThat(result).isInstanceOf(Ok::class.java)
    assertThat((result as Ok).value).isEqualTo(validator)
  }
}
