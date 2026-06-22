/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package testutils

import maru.core.SealedBeaconBlock
import maru.p2p.NoOpP2PNetwork
import maru.p2p.P2PNetwork
import maru.p2p.SealedBeaconBlockHandler
import maru.p2p.ValidationResult
import tech.pegasys.teku.infrastructure.async.SafeFuture

class InjectableSealedBlocksFakeNetwork : P2PNetwork by NoOpP2PNetwork {
  var handler: SealedBeaconBlockHandler<ValidationResult>? = null

  override fun subscribeToBlocks(subscriber: SealedBeaconBlockHandler<ValidationResult>): Int {
    handler = subscriber
    return 0
  }

  fun injectSealedBlock(sealedBlock: SealedBeaconBlock): SafeFuture<ValidationResult> =
    handler!!.handleSealedBlock(sealedBlock)
}
