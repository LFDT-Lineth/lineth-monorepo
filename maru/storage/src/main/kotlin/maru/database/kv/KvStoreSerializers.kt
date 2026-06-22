/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.database.kv

import maru.serialization.rlp.RLPSerializers

object KvStoreSerializers {
  val BytesSerializer = BytesSerializer()
  val BeaconStateSerializer = KvStoreSerializerAdapter(RLPSerializers.BeaconStateSerializer)
  val BeaconBlockSerializer = KvStoreSerializerAdapter(RLPSerializers.BeaconBlockSerializer)
  val SealedBeaconBlockSerializer = KvStoreSerializerAdapter(RLPSerializers.SealedBeaconBlockSerializer)
  val ULongSerializer = ULongSerializer()
}
