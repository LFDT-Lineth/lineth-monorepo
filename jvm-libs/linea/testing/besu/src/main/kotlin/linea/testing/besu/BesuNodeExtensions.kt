/*
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package linea.testing.besu

import org.hyperledger.besu.tests.acceptance.dsl.node.BesuNode
import org.web3j.protocol.core.DefaultBlockParameter
import org.web3j.protocol.core.methods.response.EthBlock
import java.math.BigInteger

fun BesuNode.ethGetBlockByNumber(
  blockParameter: DefaultBlockParameter,
  returnFullTransactionObjects: Boolean = false,
): EthBlock.Block =
  this
    .nodeRequests()
    .eth()
    .ethGetBlockByNumber(
      blockParameter,
      returnFullTransactionObjects,
    ).sendAsync()
    .get()
    .block

fun BesuNode.ethGetBlockByNumber(
  blockTag: String,
  returnFullTransactionObjects: Boolean = false,
): EthBlock.Block = this.ethGetBlockByNumber(DefaultBlockParameter.valueOf(blockTag), returnFullTransactionObjects)

fun BesuNode.ethGetBlockByNumber(
  blockNumber: ULong,
  returnFullTransactionObjects: Boolean = false,
): EthBlock.Block =
  this.ethGetBlockByNumber(
    DefaultBlockParameter.valueOf(BigInteger.valueOf(blockNumber.toLong())),
    returnFullTransactionObjects,
  )

fun BesuNode.latestBlock(returnFullTransactionObjects: Boolean = true): EthBlock.Block =
  this.ethGetBlockByNumber(
    DefaultBlockParameter.valueOf("latest"),
    returnFullTransactionObjects,
  )
