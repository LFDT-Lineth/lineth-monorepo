package net.consensys.zkevm.coordinator.api.dto

import net.consensys.linea.jsonrpc.JsonRpcRequestListParams
import org.junit.jupiter.api.Assertions.assertEquals
import org.junit.jupiter.api.Assertions.assertThrows
import org.junit.jupiter.api.Test

class ConflationCreateProverRequestJsonDtoTest {

  @Test
  fun `parseFrom should correctly parse valid JSON RPC request params`() {
    val jsonRpcRequest = JsonRpcRequestListParams(
      jsonrpc = "2.0",
      id = 1,
      method = "conflation_createProverRequests",
      params = listOf(
        mapOf("startBlockNumber" to 10, "endBlockNumber" to 20),
        mapOf("startBlockNumber" to 30, "endBlockNumber" to 40),
      ),
    )

    val dtoList = ConflationCreateProverRequestJsonDto.parseFrom(jsonRpcRequest)

    assertEquals(2, dtoList.size)
    assertEquals(10, dtoList[0].startBlockNumber)
    assertEquals(20, dtoList[0].endBlockNumber)
    assertEquals(30, dtoList[1].startBlockNumber)
    assertEquals(40, dtoList[1].endBlockNumber)
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
