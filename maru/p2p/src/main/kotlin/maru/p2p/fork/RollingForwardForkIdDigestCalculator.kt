/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.p2p.fork

import maru.consensus.ForkSpec
import maru.core.Hasher
import maru.core.ObjHasher
import maru.crypto.Keccak256Hasher
import maru.database.BeaconChain
import java.nio.ByteBuffer

internal fun interface ForkDigestCalculator {
  fun calculateForkDigests(forks: List<ForkSpec>): List<ForkInfo>
}

class RollingForwardForkIdDigestCalculator(
  private val chainId: UInt,
  private val beaconChain: BeaconChain,
  private val digester: ObjHasher<ForkId> = ForkIdDigester(),
) : ForkDigestCalculator {
  override fun calculateForkDigests(forks: List<ForkSpec>): List<ForkInfo> {
    val genesisBeaconBlockHash =
      beaconChain
        .getBeaconState(0UL)!!
        .beaconBlockHeader
        .hash
    val genesisForkIdDigest =
      genesisForkIdDigest(
        genesisBeaconBlockHash = genesisBeaconBlockHash,
        chainId = chainId,
        hasher = Keccak256Hasher,
      )

    return rollingForwardForkSpecsDigests(
      genesisForkIdDigest = genesisForkIdDigest,
      forks = forks,
      forkIdDigester = digester::hash,
      forkSpecDigester = ForkSpecDigester::serialize,
    )
  }

  companion object {
    internal fun genesisForkIdDigest(
      genesisBeaconBlockHash: ByteArray,
      chainId: UInt,
      hasher: Hasher,
    ): ByteArray {
      val bytes =
        ByteBuffer
          .allocate(genesisBeaconBlockHash.size + Int.SIZE_BYTES)
          .put(genesisBeaconBlockHash)
          .putInt(chainId.toInt())
          .array()
      return hasher
        .hash(bytes)
        .takeLast(4)
        .toByteArray()
    }

    internal fun rollingForwardForkSpecsDigests(
      genesisForkIdDigest: ByteArray,
      forks: List<ForkSpec>,
      forkSpecDigester: (ForkSpec) -> ByteArray,
      forkIdDigester: (ForkId) -> ByteArray,
    ): List<ForkInfo> {
      var prevForkDigest = genesisForkIdDigest
      return forks
        .sortedBy { it.timestampSeconds }
        .map { forkSpec ->
          val forkIdDigest = forkIdDigester(ForkId(prevForkDigest, forkSpecDigester(forkSpec)))
          prevForkDigest = forkIdDigest
          ForkInfo(forkSpec, forkIdDigest)
        }
    }
  }
}
