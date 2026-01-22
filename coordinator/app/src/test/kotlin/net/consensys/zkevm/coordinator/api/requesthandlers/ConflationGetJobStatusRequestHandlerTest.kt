package net.consensys.zkevm.coordinator.api.requesthandlers

import net.consensys.linea.jsonrpc.JsonRpcRequestListParams
import net.consensys.linea.jsonrpc.JsonRpcRequestMapParams
import org.junit.jupiter.api.Assertions.assertEquals
import org.junit.jupiter.api.Assertions.assertThrows
import org.junit.jupiter.api.Test

class ConflationGetJobStatusRequestHandlerTest {

  @Test
  fun `parseJobIdsFromRequest should correctly parse job IDs from request params`() {
    val requestWithListParams = JsonRpcRequestListParams(
      id = 1,
      method = ConflationGetJobStatusRequestHandler.METHOD_NAME,
      params = listOf("job1", "job2", "job3"),
      jsonrpc = "2.0",
    )
    val jobIds = ConflationGetJobStatusRequestHandler.parseJobIdsFromRequest(requestWithListParams)
    assertEquals(listOf("job1", "job2", "job3"), jobIds)

    val requestWithInvalidParams = JsonRpcRequestMapParams(
      id = 2,
      method = ConflationGetJobStatusRequestHandler.METHOD_NAME,
      params = mapOf("jobId" to "job1"),
      jsonrpc = "2.0",
    )
    assertThrows(IllegalArgumentException::class.java) {
      ConflationGetJobStatusRequestHandler.parseJobIdsFromRequest(requestWithInvalidParams)
    }

    val requestWithInvalidParams2 = JsonRpcRequestListParams(
      id = 3,
      method = ConflationGetJobStatusRequestHandler.METHOD_NAME,
      params = listOf("job1", "job2", 3),
      jsonrpc = "2.0",
    )
    assertThrows(IllegalArgumentException::class.java) {
      ConflationGetJobStatusRequestHandler.parseJobIdsFromRequest(requestWithInvalidParams2)
    }
  }
}
