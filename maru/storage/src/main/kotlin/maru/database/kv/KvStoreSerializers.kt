/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.database.kv

import maru.serialization.rlp.BeaconStateSerDe
import maru.serialization.rlp.SealedBeaconBlockSerDe as RlpSealedBeaconBlockSerDe

/**
 * KvStore (de)serializers for the beacon chain columns/variables. The block/state serializers are built
 * from the node's [maru.serialization.rlp.ForkAwareBlockHashing] so that deserialized headers carry the
 * fork-aware chain-identity hash function and their `beaconBlockIdHash` matches the key they were stored under.
 */
class KvStoreSerializers(
  beaconStateSerializer: BeaconStateSerDe,
  sealedBeaconBlockSerializer: RlpSealedBeaconBlockSerDe,
) {
  val bytesSerializer = BytesSerializer()
  val beaconStateSerializer = KvStoreSerializerAdapter(beaconStateSerializer)
  val sealedBeaconBlockSerializer = KvStoreSerializerAdapter(sealedBeaconBlockSerializer)
  val uLongSerializer = ULongSerializer()
}
