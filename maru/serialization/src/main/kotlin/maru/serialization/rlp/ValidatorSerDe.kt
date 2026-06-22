/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.serialization.rlp

import maru.core.Validator
import org.apache.tuweni.bytes.Bytes
import org.hyperledger.besu.ethereum.rlp.RLPInput
import org.hyperledger.besu.ethereum.rlp.RLPOutput

class ValidatorSerDe : RLPSerDe<Validator> {
  override fun writeTo(
    value: Validator,
    rlpOutput: RLPOutput,
  ) {
    rlpOutput.startList()
    rlpOutput.writeBytes(Bytes.wrap(value.address))
    rlpOutput.endList()
  }

  override fun readFrom(rlpInput: RLPInput): Validator {
    rlpInput.enterList()
    val address: ByteArray = rlpInput.readBytes().toArray()
    rlpInput.leaveList()
    return Validator(address)
  }
}
