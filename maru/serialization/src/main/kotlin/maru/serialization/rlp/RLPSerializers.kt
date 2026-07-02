/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.serialization.rlp

/**
 * Static, fork-free serializers only:
 *  - header-free SerDes (no header inside → safe to serialize AND deserialize statically), and
 *  - write-only serializers for header-containing types (serializing never touches the fork-aware hash
 *    function, so it is safe; these are the single source of truth for the byte layout and back [HashUtil]).
 *
 * Deserializing a header-containing type MUST inject the node's fork-aware `headerHashFunction`, so the full
 * header-containing SerDes are NOT exposed here — they come only from [ForkAwareBlockHashing]. Exposing a
 * static one with a round-inclusive default would be a latent QBFT_PHASE1 bug.
 */
object RLPSerializers {
  val ValidatorSerializer = ValidatorSerDe()
  val SealSerializer = SealSerDe()
  val ExecutionPayloadSerializer = ExecutionPayloadSerDe()
  val BeaconBlockBodySerializer =
    BeaconBlockBodySerDe(
      sealSerializer = SealSerializer,
      executionPayloadSerializer = ExecutionPayloadSerializer,
    )

  // Write-only serializers for header-containing types (function-free, single source of byte layout).
  val BeaconBlockHeaderRLPSerializer = BeaconBlockHeaderRLPSerializer(ValidatorSerializer)
  val BeaconStateRLPSerializer = BeaconStateRLPSerializer(BeaconBlockHeaderRLPSerializer, ValidatorSerializer)
  val BeaconBlockRLPSerializer = BeaconBlockRLPSerializer(BeaconBlockHeaderRLPSerializer, BeaconBlockBodySerializer)
  val SealedBeaconBlockRLPSerializer = SealedBeaconBlockRLPSerializer(BeaconBlockRLPSerializer, SealSerializer)
  val SealedBeaconBlockCompressorRLPSerializer =
    MaruCompressorRLPSerializer(serializer = SealedBeaconBlockRLPSerializer)
}
