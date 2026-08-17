/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.test.extensions

import maru.test.cluster.MaruCluster
import org.assertj.core.api.Assertions.fail

fun MaruCluster.nodesHeadBlockNumbers(): Map<String, Pair<ULong, ULong?>> =
  this.nodes.associate {
    it.label to (it.maru.headBeaconBlockNumber() to it.elNode?.headBlockNumber())
  }

fun MaruCluster.assertNodesAreSyncedUpTo(targetBlockNumber: ULong) {
  val nodesHeadBlockNumbers = nodesHeadBlockNumbers()
  val inSync =
    nodesHeadBlockNumbers.values.all { (maruHead, besuHead) ->
      maruHead >= targetBlockNumber && (besuHead?.let { it >= targetBlockNumber } ?: true)
    }

  if (!inSync) {
    fail<Unit>("Nodes did not sync to block $targetBlockNumber nodes heads: $nodesHeadBlockNumbers")
  }
}
