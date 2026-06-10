package linea.web3j.ethapi

import com.fasterxml.jackson.databind.JsonNode
import linea.ethapi.ExecutionWitness
import linea.ethapi.ExecutionWitnessClientException
import linea.ethapi.ExecutionWitnessError
import linea.kotlin.decodeHex

object ExecutionWitnessResponseParser {

  fun parse(json: JsonNode): ExecutionWitness {
    return try {
      ExecutionWitness(
        state = parseHexList(json, "state"),
        keys = parseHexList(json, "keys"),
        codes = parseHexList(json, "codes"),
        headers = parseHexList(json, "headers"),
      )
    } catch (throwable: Throwable) {
      throw ExecutionWitnessClientException(
        ExecutionWitnessError.PARSE_ERROR,
        throwable.message ?: "failed to parse execution witness",
        throwable,
      )
    }
  }

  private fun parseHexList(json: JsonNode, field: String): List<ByteArray> {
    val array = json.get(field)
      ?: throw IllegalArgumentException("missing or invalid field: $field")
    if (!array.isArray) {
      throw IllegalArgumentException("missing or invalid field: $field")
    }
    return array.map { element ->
      element.asText().decodeHex()
    }
  }
}
