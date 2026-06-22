/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus

import maru.consensus.qbft.DifficultyAwareQbftFactory
import maru.core.Protocol

interface ProtocolFactory {
  fun create(forkSpec: ForkSpec): Protocol
}

class OmniProtocolFactory(
  private val qbftConsensusFactory: ProtocolFactory,
  private val difficultyAwareQbftFactory: DifficultyAwareQbftFactory,
) : ProtocolFactory {
  override fun create(forkSpec: ForkSpec): Protocol =
    when (forkSpec.configuration) {
      is QbftConsensusConfig -> {
        qbftConsensusFactory.create(forkSpec)
      }

      is DifficultyAwareQbftConfig -> {
        difficultyAwareQbftFactory.create(forkSpec)
      }

      else -> {
        throw IllegalArgumentException("Fork $forkSpec is unknown!")
      }
    }
}
