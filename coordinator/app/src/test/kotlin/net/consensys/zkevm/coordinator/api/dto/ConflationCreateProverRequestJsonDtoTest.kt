package net.consensys.zkevm.coordinator.api.dto

import net.consensys.linea.jsonrpc.JsonRpcRequestListParams
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Assertions.assertEquals
import org.junit.jupiter.api.Assertions.assertThrows
import org.junit.jupiter.api.Test

class ConflationCreateProverRequestJsonDtoTest {

  @Test
  fun `parseFrom should correctly parse valid JSON RPC request params`() {
    val expectedDtoList = listOf(
      ConflationCreateProverRequestJsonDto(
        startBlockNumber = 10,
        endBlockNumber = 20,
        blobCompressorVersion = "V1_2",
        batchesFixedSize = null,
        parentBlobShnarf = null,
        tracesApi = TracesApiDto(
          endpoint = "https://example.com/traces",
          version = "v1",
          requestLimitPerEndpoint = 100,
        ),
        shomeiApi = ShomeiApiDto(
          endpoint = "https://example.com/shomei",
          version = "v1",
          requestLimitPerEndpoint = 50,
        ),
      ),
      ConflationCreateProverRequestJsonDto(
        startBlockNumber = 30,
        endBlockNumber = 40,
        blobCompressorVersion = "V2",
        batchesFixedSize = null,
        parentBlobShnarf = null,
        tracesApi = TracesApiDto(
          endpoint = "https://example.com/traces",
          version = "v2",
          requestLimitPerEndpoint = 100,
        ),
        shomeiApi = ShomeiApiDto(
          endpoint = "https://example.com/shomei",
          version = "v2",
          requestLimitPerEndpoint = 50,
        ),

      ),
    )
    val jsonRpcRequest = JsonRpcRequestListParams(
      jsonrpc = "2.0",
      id = 1,
      method = "conflation_createProverRequests",
      params = listOf(
        mapOf(
          "startBlockNumber" to 10,
          "endBlockNumber" to 20,
          "blobCompressorVersion" to "V1_2",
          "tracesApi" to mapOf(
            "endpoint" to "https://example.com/traces",
            "version" to "v1",
            "requestLimitPerEndpoint" to 100,
          ),
          "shomeiApi" to mapOf(
            "endpoint" to "https://example.com/shomei",
            "version" to "v1",
            "requestLimitPerEndpoint" to 50,
          ),
        ),
        mapOf(
          "startBlockNumber" to 30,
          "endBlockNumber" to 40,
          "blobCompressorVersion" to "V2",
          "tracesApi" to mapOf(
            "endpoint" to "https://example.com/traces",
            "version" to "v2",
            "requestLimitPerEndpoint" to 100,
          ),
          "shomeiApi" to mapOf(
            "endpoint" to "https://example.com/shomei",
            "version" to "v2",
            "requestLimitPerEndpoint" to 50,
          ),
        ),
      ),
    )

    val dtoList = ConflationCreateProverRequestJsonDto.parseFrom(jsonRpcRequest)

    assertEquals(2, dtoList.size)
    assertThat(dtoList).isEqualTo(expectedDtoList)
  }

  @Test
  fun `parseFrom should throw exception for invalid JSON RPC request params`() {
    val jsonRpcRequest = JsonRpcRequestListParams(
      jsonrpc = "2.0",
      id = 1,
      method = "conflation_createProverRequests",
      params = listOf(
        mapOf("startBlockNumber" to "invalid", "endBlockNumber" to 20),
      ),
    )

    assertThrows(IllegalArgumentException::class.java) {
      ConflationCreateProverRequestJsonDto.parseFrom(jsonRpcRequest)
    }
  }
}
