/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.test.extensions

import linea.kotlin.toULong
import maru.test.cluster.BesuCluster
import org.hyperledger.besu.tests.acceptance.dsl.node.BesuNode
import org.web3j.protocol.core.DefaultBlockParameter
import org.web3j.protocol.core.methods.response.EthBlock

fun BesuCluster.nodeHeads(): Map<String, ULong> = this.nodes.mapValues { it.value.latestBlockNumber() }

fun BesuNode.latestBlock(includeTransactions: Boolean = false): EthBlock.Block {
  // Besu sometime returns null for latest block, when at start up retry a few times to avoid flaky teste
  var retries = 5
  var block: EthBlock.Block? = null
  while (retries > 0 && block == null) {
    try {
      block = this
        .nodeRequests()
        .eth()
        .ethGetBlockByNumber(DefaultBlockParameter.valueOf("latest"), includeTransactions)
        .send()
        .block
    } catch (e: Exception) {
      // ignore and retry
    }
    if (block == null) {
      retries--
      Thread.sleep(200)
    }
  }
  return block ?: throw RuntimeException("Could not retrieve latest block after retries")
}

fun BesuNode.latestBlockNumber(): ULong =
  this
    .nodeRequests()
    .eth()
    .ethBlockNumber()
    .send()
    .blockNumber
    .toULong()
