package linea.web3j.ethapi

import linea.ethapi.ExecutionWitness
import linea.kotlin.decodeHex
import tools.jackson.databind.JsonNode

object ExecutionWitnessResponseParser {

  fun parse(json: JsonNode): ExecutionWitness {
    return ExecutionWitness(
      state = parseHexList(json, "state"),
      codes = parseHexList(json, "codes"),
      headers = parseHexList(json, "headers"),
    )
  }

  private fun parseHexList(json: JsonNode, field: String): List<ByteArray> {
    val array = json.get(field)
      ?: throw IllegalArgumentException("missing or invalid field: $field")
    if (!array.isArray) {
      throw IllegalArgumentException("missing or invalid field: $field")
    }
    return (array as Iterable<JsonNode>).map { element ->
      element.asString().decodeHex()
    }
  }
}
