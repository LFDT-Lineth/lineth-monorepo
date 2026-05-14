package linea.coordinator.api.requesthandlers

import com.github.michaelbull.result.Err
import com.github.michaelbull.result.Ok
import io.vertx.core.json.JsonObject
import linea.coordinator.app.conflationbacktesting.ConflationBacktestingService
import net.consensys.linea.jsonrpc.JsonRpcRequestListParams
import net.consensys.linea.jsonrpc.JsonRpcRequestMapParams
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import org.mockito.kotlin.mock
import org.mockito.kotlin.verify
import org.mockito.kotlin.whenever
import tech.pegasys.teku.infrastructure.async.SafeFuture

class ConflationStopJobRequestHandlerTest {

  private val emptyRequestJson = JsonObject()

  @Test
  fun `invoke stops requested job and returns STOPPED on success`() {
    val service = mock<ConflationBacktestingService>()
    whenever(service.stopConflationBacktestingJob("job1")).thenReturn(SafeFuture.completedFuture(Unit))
    val handler = ConflationStopJobRequestHandler(service)

    val request = JsonRpcRequestListParams(
      id = 1,
      method = ConflationStopJobRequestHandler.METHOD_NAME,
      params = listOf("job1"),
      jsonrpc = "2.0",
    )

    val response = handler.invoke(null, request, emptyRequestJson).toCompletionStage().toCompletableFuture().get()

    assertThat(response).isInstanceOf(Ok::class.java)
    val success = (response as Ok).value
    assertThat(success.id).isEqualTo(1)
    assertThat(success.result).isEqualTo(ConflationStopJobRequestHandler.STOPPED_RESULT)
    verify(service).stopConflationBacktestingJob("job1")
  }

  @Test
  fun `invoke returns ERROR when service throws synchronously`() {
    val service = mock<ConflationBacktestingService>()
    whenever(service.stopConflationBacktestingJob("missing"))
      .thenThrow(IllegalArgumentException("No in-progress conflation backtesting job found with jobId=missing"))
    val handler = ConflationStopJobRequestHandler(service)

    val request = JsonRpcRequestListParams(
      id = 2,
      method = ConflationStopJobRequestHandler.METHOD_NAME,
      params = listOf("missing"),
      jsonrpc = "2.0",
    )

    val response = handler.invoke(null, request, emptyRequestJson).toCompletionStage().toCompletableFuture().get()

    assertThat(response).isInstanceOf(Ok::class.java)
    val outcome = (response as Ok).value.result as String
    assertThat(outcome).startsWith("ERROR:")
    assertThat(outcome).contains("missing")
  }

  @Test
  fun `invoke returns ERROR when stop future fails`() {
    val service = mock<ConflationBacktestingService>()
    val failingFuture: SafeFuture<Unit> = SafeFuture.failedFuture(RuntimeException("boom"))
    whenever(service.stopConflationBacktestingJob("job1")).thenReturn(failingFuture)
    val handler = ConflationStopJobRequestHandler(service)

    val request = JsonRpcRequestListParams(
      id = 3,
      method = ConflationStopJobRequestHandler.METHOD_NAME,
      params = listOf("job1"),
      jsonrpc = "2.0",
    )

    val response = handler.invoke(null, request, emptyRequestJson).toCompletionStage().toCompletableFuture().get()

    assertThat(response).isInstanceOf(Ok::class.java)
    assertThat((response as Ok).value.result).isEqualTo("ERROR: boom")
  }

  @Test
  fun `invoke returns invalidParams when params are not a list`() {
    val service = mock<ConflationBacktestingService>()
    val handler = ConflationStopJobRequestHandler(service)

    val request = JsonRpcRequestMapParams(
      id = 4,
      method = ConflationStopJobRequestHandler.METHOD_NAME,
      params = mapOf("jobId" to "job1"),
      jsonrpc = "2.0",
    )

    val response = handler.invoke(null, request, emptyRequestJson).toCompletionStage().toCompletableFuture().get()

    assertThat(response).isInstanceOf(Err::class.java)
  }

  @Test
  fun `invoke returns invalidParams when more than one job ID is given`() {
    val service = mock<ConflationBacktestingService>()
    val handler = ConflationStopJobRequestHandler(service)

    val request = JsonRpcRequestListParams(
      id = 5,
      method = ConflationStopJobRequestHandler.METHOD_NAME,
      params = listOf("job1", "job2"),
      jsonrpc = "2.0",
    )

    val response = handler.invoke(null, request, emptyRequestJson).toCompletionStage().toCompletableFuture().get()

    assertThat(response).isInstanceOf(Err::class.java)
  }
}
