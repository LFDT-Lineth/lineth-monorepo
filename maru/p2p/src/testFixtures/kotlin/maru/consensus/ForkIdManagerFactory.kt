/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus

import maru.core.ext.DataGenerators
import maru.database.BeaconChain
import maru.p2p.fork.ForkPeeringManager
import maru.p2p.fork.LenientForkPeeringManager
import java.time.Clock
import kotlin.time.Duration
import kotlin.time.Duration.Companion.minutes

object ForkIdManagerFactory {
  fun createForkIdHashManager(
    chainId: UInt,
    beaconChain: BeaconChain,
    elFork: ElFork = ElFork.Prague,
    consensusConfig: ConsensusConfig =
      QbftConsensusConfig(
        validatorSet = setOf(
          DataGenerators.randomValidator(),
          DataGenerators.randomValidator(),
        ),
        fork = ChainFork(ClFork.QBFT_PHASE0, elFork = elFork),
      ),
    forks: List<ForkSpec> = listOf(ForkSpec(0UL, 1u, consensusConfig)),
    peeringForkMismatchLeewayTime: Duration = 5.minutes,
    clock: Clock = Clock.systemUTC(),
  ): ForkPeeringManager =
    LenientForkPeeringManager.create(
      chainId = chainId,
      beaconChain = beaconChain,
      forks = forks,
      peeringForkMismatchLeewayTime = peeringForkMismatchLeewayTime,
      clock = clock,
    )
}
