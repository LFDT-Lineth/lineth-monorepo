package net.consensys.zkevm.domain

import kotlin.time.Instant

/**
 * L2 start-block time for a proof span. For invalidity proofs this is the simulated execution block time.
 */
interface StartBlockTimestampProvider {
  val startBlockTimestamp: Instant
}
