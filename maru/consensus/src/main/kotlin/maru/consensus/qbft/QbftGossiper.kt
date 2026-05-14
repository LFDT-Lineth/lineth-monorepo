/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus.qbft

import org.hyperledger.besu.consensus.qbft.core.types.QbftGossiper
import org.hyperledger.besu.consensus.qbft.core.types.QbftMessage

/**
 * Gossiper for libp2p-based QBFT consensus.
 *
 * This implementation is intentionally a no-op for all messages:
 * - Non-replayed messages (locally created): Besu's round/height manager calls
 *   ValidatorMulticaster directly when creating PROPOSE/PREPARE/COMMIT messages. LibP2P
 *   flood-publish handles propagation to peers.
 * - Replayed messages (drained from the future buffer): these were received from P2P peers and
 *   were already propagated by libp2p at that time. Their content is still in libp2p's seen
 *   cache, so re-publishing them would fail with MessageAlreadySeenException.
 */
class QbftGossiper : QbftGossiper {
  override fun send(
    message: QbftMessage,
    replayed: Boolean,
  ) {
    Unit
  }
}
