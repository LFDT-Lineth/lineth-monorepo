/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.serialization.rlp

import maru.core.Seal
import org.apache.tuweni.bytes.Bytes
import org.hyperledger.besu.ethereum.rlp.RLPInput
import org.hyperledger.besu.ethereum.rlp.RLPOutput

class SealSerDe : RLPSerDe<Seal> {
  override fun writeTo(
    value: Seal,
    rlpOutput: RLPOutput,
  ) {
    rlpOutput.startList()
    rlpOutput.writeBytes(Bytes.wrap(value.signature))
    rlpOutput.endList()
  }

  override fun readFrom(rlpInput: RLPInput): Seal {
    rlpInput.enterList()
    val signature = rlpInput.readBytes().toArray()
    rlpInput.leaveList()
    return Seal(signature)
  }
}
