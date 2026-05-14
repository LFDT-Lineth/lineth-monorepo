/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.serialization.rlp

import maru.core.HashUtil
import maru.crypto.Hashing

object RLPSerializers {
  val ValidatorSerializer = ValidatorSerDe()

  val BeaconBlockHeaderSerializer =
    BeaconBlockHeaderSerDe(
      validatorSerializer = ValidatorSerializer,
      hasher = Hashing::keccak,
      headerHashFunction = HashUtil::headerHash,
    )
  val SealSerializer = SealSerDe()
  val ExecutionPayloadSerializer = ExecutionPayloadSerDe()
  val BeaconBlockBodySerializer =
    BeaconBlockBodySerDe(
      sealSerializer = SealSerializer,
      executionPayloadSerializer = ExecutionPayloadSerializer,
    )
  val BeaconBlockSerializer =
    BeaconBlockSerDe(
      beaconBlockHeaderSerializer = BeaconBlockHeaderSerializer,
      beaconBlockBodySerializer = BeaconBlockBodySerializer,
    )
  val SealedBeaconBlockSerializer =
    SealedBeaconBlockSerDe(
      beaconBlockSerializer = BeaconBlockSerializer,
      sealSerializer = SealSerializer,
    )
  val SealedBeaconBlockCompressorSerializer =
    MaruCompressorRLPSerDe(
      serDe = SealedBeaconBlockSerializer,
    )
  val BeaconStateSerializer =
    BeaconStateSerDe(
      beaconBlockHeaderSerializer = BeaconBlockHeaderSerializer,
      validatorSerializer = ValidatorSerializer,
    )
  val DefaultHeaderHashFunction = HashUtil.headerHash(BeaconBlockHeaderSerializer, Hashing::keccak)
}
