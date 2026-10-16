/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.test.extensions

import maru.test.cluster.BesuElNode
import maru.test.cluster.ClusterNode
import maru.test.cluster.MaruCluster
import org.assertj.core.api.Assertions.fail
import org.web3j.protocol.core.methods.response.EthBlock

fun MaruCluster.besuBlocks(blockNumber: ULong): Map<ClusterNode, EthBlock.Block> {
  return this
    .nodesWithBesuEl()
    .associateBy { it }
    .mapValues { kv -> (kv.value.elNode as BesuElNode).besu.getBlockByNumber(blockNumber) }
}

fun MaruCluster.nodesHeadBlockNumbers(): Map<String, Pair<ULong, ULong?>> =
  this.nodes.associate {
    it.label to (it.maru.headBeaconBlockNumber() to it.elNode?.headBlockNumber())
  }

fun MaruCluster.assertNodesAreSyncedUpTo(targetBlockNumber: ULong, expectBeaconChain: Boolean = true) {
  val nodesHeadBlockNumbers = nodesHeadBlockNumbers()
  val inSync =
    nodesHeadBlockNumbers.values.all { (maruHead, besuHead) ->
      var nodeInSync = if (expectBeaconChain) maruHead >= targetBlockNumber else true
      nodeInSync = nodeInSync && besuHead?.let { it >= targetBlockNumber } ?: true
      nodeInSync
    }

  if (!inSync) {
    fail<Unit>("Nodes did not sync to block $targetBlockNumber nodes heads: $nodesHeadBlockNumbers")
  }
}
