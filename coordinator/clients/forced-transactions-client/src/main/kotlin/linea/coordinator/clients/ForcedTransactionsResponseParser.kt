package linea.coordinator.clients

import com.fasterxml.jackson.databind.node.ArrayNode
import com.fasterxml.jackson.databind.node.NullNode
import com.fasterxml.jackson.databind.node.ObjectNode
import linea.forcedtx.ForcedTransactionInclusionResult
import linea.forcedtx.ForcedTransactionInclusionStatus
import linea.forcedtx.ForcedTransactionResponse
import linea.kotlin.decodeHex
import linea.kotlin.toULongFromHex

object ForcedTransactionsResponseParser {

  /**
   * Parses the response from linea_sendForcedRawTransaction RPC call.
   *
   * Expected response format:
   * ```json
   * [
   *   {"forcedTransactionNumber": 6, "hash": "0xabc123..."},
   *   {"forcedTransactionNumber": 7, "error": "Invalid nonce"}
   * ]
   * ```
   */
  fun parseSendForcedRawTransactionResponse(result: Any?): List<ForcedTransactionResponse> {
    if (result == null || result is NullNode) {
      return emptyList()
    }

    val resultArray = result as ArrayNode
    return resultArray.map { item ->
      val jsonObj = item as ObjectNode
      val ftxNumber = jsonObj.get("forcedTransactionNumber").asLong().toULong()
      val hash = jsonObj.get("hash")?.takeIf { !it.isNull }?.asText()?.decodeHex()
      val error = jsonObj.get("error")?.takeIf { !it.isNull }?.asText()

      ForcedTransactionResponse(
        ftxNumber = ftxNumber,
        ftxHash = hash,
        ftxError = error,
      )
    }
  }

  /**
   * Parses the response from linea_getForcedTransactionInclusionStatus RPC call.
   *
   * Expected response format (found):
   * ```json
   * {
   *   "forcedTransactionNumber": 6,
   *   "blockNumber": "0xeff35f",
   *   "blockTimestamp": 1234567890,
   *   "from": "0x6221a9c005f6e47eb398fd867784cacfdcfff4e7",
   *   "inclusionResult": "Included",
   *   "transactionHash": "0xabc123..."
   * }
   * ```
   *
   * Returns null if result is null (transaction not found).
   */
  fun parseForcedTransactionInclusionStatus(result: Any?): ForcedTransactionInclusionStatus? {
    if (result == null || result is NullNode) {
      return null
    }

    val jsonObj = result as ObjectNode
    val ftxNumber = jsonObj.get("forcedTransactionNumber").asLong().toULong()
    val blockNumber = jsonObj.get("blockNumber").asText().toULongFromHex()
    val blockTimestamp = jsonObj.get("blockTimestamp").asLong()
    val from = jsonObj.get("from").asText().decodeHex()
    val inclusionResultStr = jsonObj.get("inclusionResult").asText()
    val transactionHash = jsonObj.get("transactionHash").asText().decodeHex()

    val inclusionResult = parseInclusionResult(inclusionResultStr)

    return ForcedTransactionInclusionStatus(
      ftxNumber = ftxNumber,
      blockNumber = blockNumber,
      blockTimestamp = java.time.Instant.ofEpochSecond(blockTimestamp).toKotlinInstant(),
      inclusionResult = inclusionResult,
      ftxHash = transactionHash,
      from = from,
    )
  }

  private fun parseInclusionResult(inclusionResultStr: String): ForcedTransactionInclusionResult {
    return try {
      ForcedTransactionInclusionResult.valueOf(inclusionResultStr)
    } catch (e: IllegalArgumentException) {
      throw IllegalArgumentException(
        "Unknown inclusion result: '$inclusionResultStr'. " +
          "Valid values: ${ForcedTransactionInclusionResult.entries.joinToString(", ")}",
        e,
      )
    }
  }

  private fun java.time.Instant.toKotlinInstant(): kotlin.time.Instant {
    return kotlin.time.Instant.fromEpochSeconds(this.epochSecond, this.nano.toLong())
  }
}
