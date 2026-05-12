/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package linea.plugin.acc.test.rpc

import com.fasterxml.jackson.annotation.JsonCreator
import com.fasterxml.jackson.annotation.JsonProperty
import org.hyperledger.besu.tests.acceptance.dsl.transaction.NodeRequests
import org.hyperledger.besu.tests.acceptance.dsl.transaction.Transaction
import org.web3j.protocol.core.Request
import org.web3j.protocol.core.Response
import java.io.IOException

/**
 * Parameters for linea_sendForcedRawTransaction RPC call.
 */
data class ForcedTransactionParam(
  val forcedTransactionNumber: Long,
  val transaction: String,
  val deadlineBlockNumber: String,
)

/**
 * Request to send forced transactions.
 */
class SendForcedRawTransactionRequest(
  private val params: List<ForcedTransactionParam>,
) : Transaction<SendForcedRawTransactionResponse> {

  override fun execute(nodeRequests: NodeRequests): SendForcedRawTransactionResponse {
    return try {
      Request(
        "linea_sendForcedRawTransaction",
        listOf(params),
        nodeRequests.web3jService,
        SendForcedRawTransactionResponse::class.java,
      ).send()
    } catch (e: IOException) {
      throw RuntimeException(e)
    }
  }
}

/**
 * Response item from linea_sendForcedRawTransaction RPC call.
 */
data class ForcedTransactionResponseItem @JsonCreator constructor(
  @param:JsonProperty("forcedTransactionNumber") val forcedTransactionNumber: Long,
  @param:JsonProperty("hash") val hash: String?,
  @param:JsonProperty("error") val error: String?,
)

/**
 * Response from linea_sendForcedRawTransaction RPC call.
 * Contains an array of response items with forcedTransactionNumber, hash, and optional error.
 */
class SendForcedRawTransactionResponse : Response<List<ForcedTransactionResponseItem>>()

/**
 * Request to get forced transaction inclusion status.
 */
class GetForcedTransactionInclusionStatusRequest(
  private val forcedTransactionNumber: Long,
) : Transaction<GetForcedTransactionInclusionStatusResponse> {

  override fun execute(nodeRequests: NodeRequests): GetForcedTransactionInclusionStatusResponse {
    return try {
      Request(
        "linea_getForcedTransactionInclusionStatus",
        listOf(forcedTransactionNumber),
        nodeRequests.web3jService,
        GetForcedTransactionInclusionStatusResponse::class.java,
      ).send()
    } catch (e: IOException) {
      throw RuntimeException(e)
    }
  }
}

/**
 * Response from linea_getForcedTransactionInclusionStatus RPC call.
 */
class GetForcedTransactionInclusionStatusResponse :
  Response<GetForcedTransactionInclusionStatusResponse.InclusionStatus>() {

  data class InclusionStatus @JsonCreator constructor(
    @param:JsonProperty("forcedTransactionNumber") val forcedTransactionNumber: Long,
    @param:JsonProperty("blockNumber") val blockNumber: String,
    @param:JsonProperty("blockTimestamp") val blockTimestamp: Long,
    @param:JsonProperty("from") val from: String,
    @param:JsonProperty("inclusionResult") val inclusionResult: String,
    @param:JsonProperty("transactionHash") val transactionHash: String,
  )
}
