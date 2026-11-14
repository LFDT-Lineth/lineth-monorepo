/*
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.executionlayer.client

import com.fasterxml.jackson.databind.JsonNode
import com.fasterxml.jackson.databind.ObjectMapper
import com.fasterxml.jackson.databind.node.ObjectNode
import com.fasterxml.jackson.dataformat.yaml.YAMLFactory
import com.fasterxml.jackson.module.kotlin.jacksonObjectMapper
import com.networknt.schema.JsonSchema
import com.networknt.schema.JsonSchemaFactory
import com.networknt.schema.SpecVersion
import java.util.concurrent.ConcurrentHashMap

/**
 * Validates Engine API params and results against the execution-apis schemas vendored in
 * `engine-api/execution-apis`.
 *
 * Each method is mapped to its params and result schemas as in `src/engine/openrpc/methods` upstream, with two
 * deliberate differences:
 * - The optional payload attributes of `engine_forkchoiceUpdated` also accept `null`, which is how clients omit them.
 * - Where upstream accepts `oneOf` V1 or V2 payloads, the V2 schema is required: the schemas allow additional
 *   properties, so a V2 payload matches both and `oneOf` would always fail.
 */
object EngineApiSpec {
  private const val SCHEMAS_PATH = "/engine-api/execution-apis/src"
  private val jsonMapper = jacksonObjectMapper()
  private val yamlMapper = ObjectMapper(YAMLFactory())
  private val schemaFactory = JsonSchemaFactory.getInstance(SpecVersion.VersionFlag.V202012)

  private val componentSchemas: ObjectNode =
    jsonMapper.createObjectNode().apply {
      listOf(
        "schemas/base-types.yaml",
        "engine/openrpc/schemas/payload.yaml",
        "engine/openrpc/schemas/forkchoice.yaml",
      ).forEach { setAll<ObjectNode>(loadYaml("$SCHEMAS_PATH/$it")) }
    }

  private fun loadYaml(resource: String): ObjectNode {
    val stream = requireNotNull(EngineApiSpec::class.java.getResourceAsStream(resource)) { "missing $resource" }
    return stream.use { yamlMapper.readTree(it) as ObjectNode }
  }

  private fun ref(schema: String) = """{"${'$'}ref":"#/components/schemas/$schema"}"""

  private fun arrayOf(schema: String) = """{"type":"array","items":${ref(schema)}}"""

  private fun orNull(schema: String) = """{"oneOf":[$schema,{"type":"null"}]}"""

  private fun getPayloadResponse(payload: String, blobsBundle: String?, withExecutionRequests: Boolean): String {
    val properties =
      buildList {
        add(""""executionPayload":${ref(payload)}""")
        add(""""blockValue":${ref("uint256")}""")
        if (blobsBundle != null) {
          add(""""blobsBundle":${ref(blobsBundle)}""")
          add(""""shouldOverrideBuilder":{"type":"boolean"}""")
        }
        if (withExecutionRequests) add(""""executionRequests":${arrayOf("bytes")}""")
      }
    val required = properties.map { it.substringBefore(":") }
    return """{"type":"object","required":$required,"properties":{${properties.joinToString(",")}}}"""
  }

  private val newPayloadV3Params = listOf(ref("ExecutionPayloadV3"), arrayOf("hash32"), ref("hash32"))

  private val paramSchemas: Map<String, List<String>> =
    mapOf(
      "engine_newPayloadV1" to listOf(ref("ExecutionPayloadV1")),
      "engine_newPayloadV2" to listOf(ref("ExecutionPayloadV2")),
      "engine_newPayloadV3" to newPayloadV3Params,
      "engine_newPayloadV4" to newPayloadV3Params + arrayOf("bytes"),
      "engine_newPayloadV5" to listOf(ref("ExecutionPayloadV4"), arrayOf("hash32"), ref("hash32"), arrayOf("bytes")),
      "engine_forkchoiceUpdatedV1" to listOf(ref("ForkchoiceStateV1"), orNull(ref("PayloadAttributesV1"))),
      "engine_forkchoiceUpdatedV2" to listOf(ref("ForkchoiceStateV1"), orNull(ref("PayloadAttributesV2"))),
      "engine_forkchoiceUpdatedV3" to listOf(ref("ForkchoiceStateV1"), orNull(ref("PayloadAttributesV3"))),
      "engine_forkchoiceUpdatedV4" to listOf(ref("ForkchoiceStateV1"), orNull(ref("PayloadAttributesV4"))),
    ) + (1..6).associate { version -> "engine_getPayloadV$version" to listOf(ref("bytes8")) }

  private val resultSchemas: Map<String, String> =
    mapOf(
      "engine_newPayloadV1" to ref("PayloadStatusV1"),
      "engine_getPayloadV1" to ref("ExecutionPayloadV1"),
      "engine_getPayloadV2" to getPayloadResponse("ExecutionPayloadV2", null, withExecutionRequests = false),
      "engine_getPayloadV3" to getPayloadResponse("ExecutionPayloadV3", "BlobsBundleV1", withExecutionRequests = false),
      "engine_getPayloadV4" to getPayloadResponse("ExecutionPayloadV3", "BlobsBundleV1", withExecutionRequests = true),
      "engine_getPayloadV5" to getPayloadResponse("ExecutionPayloadV3", "BlobsBundleV2", withExecutionRequests = true),
      "engine_getPayloadV6" to getPayloadResponse("ExecutionPayloadV4", "BlobsBundleV2", withExecutionRequests = true),
    ) +
      (2..5).associate { version -> "engine_newPayloadV$version" to ref("PayloadStatusNoInvalidBlockHash") } +
      (1..4).associate { version -> "engine_forkchoiceUpdatedV$version" to ref("ForkchoiceUpdatedResponseV1") }

  private val schemaCache = ConcurrentHashMap<String, JsonSchema>()

  private fun schema(schemaJson: String): JsonSchema =
    schemaCache.computeIfAbsent(schemaJson) {
      val root = jsonMapper.readTree(schemaJson) as ObjectNode
      root.putObject("components").set<ObjectNode>("schemas", componentSchemas)
      schemaFactory.getSchema(root)
    }

  /** Spec violations of the params of a request to [method], empty when they conform. */
  fun paramsViolations(method: String, params: JsonNode): List<String> {
    val schemas = requireNotNull(paramSchemas[method]) { "no params schema for $method" }
    if (params.size() != schemas.size) return listOf("$method expects ${schemas.size} params, got ${params.size()}")
    return schemas.flatMapIndexed { index, schemaJson ->
      schema(schemaJson).validate(params[index]).map { "param $index: ${it.message}" }
    }
  }

  /** Spec violations of a [result] of [method], empty when it conforms. */
  fun resultViolations(method: String, result: JsonNode): List<String> {
    val schemaJson = requireNotNull(resultSchemas[method]) { "no result schema for $method" }
    return schema(schemaJson).validate(result).map { it.message }
  }
}
