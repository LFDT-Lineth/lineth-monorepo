package lineth.coordinator.clients.prover

import com.fasterxml.jackson.databind.ObjectMapper
import com.github.michaelbull.result.Ok
import io.vertx.core.Vertx
import io.vertx.core.buffer.Buffer
import linea.domain.BlockInterval
import linea.domain.ProofIndex
import linea.error.ErrorResponse
import net.consensys.linea.httprest.client.HttpRestClient
import net.consensys.linea.httprest.client.RestErrorType
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Duration.Companion.seconds
import kotlin.time.Instant

class RestfulProverProofTransportTest {

  @Test
  fun `removeRequests omits null criteria fields from the request body`() {
    val restClient = RecordingHttpRestClient()
    val transport = RestfulProverProofTransport<Any, Any, DummyProofIndex>(
      restClient = restClient,
      vertx = Vertx.vertx(),
      chainId = 1,
      proofType = "execution",
      startBlockProvider = { it.startBlockNumber },
      endBlockProvider = { it.endBlockNumber },
      responseDtoClass = Any::class.java,
      pollingInterval = 1.seconds,
      pollingTimeout = 1.seconds,
      objectMapper = ObjectMapper(),
    )

    transport.removeRequests(null).get()

    val body = ObjectMapper().readTree(checkNotNull(restClient.lastPostBody))
    val criteria = body.get("criteria")
    assertThat(criteria).isNotNull
    assertThat(criteria.has("start_block_gte")).isFalse()
    assertThat(criteria.get("proof_type").asText()).isEqualTo("execution")
  }

  @Test
  fun `removeRequests includes startBlockGte when it is set`() {
    val restClient = RecordingHttpRestClient()
    val transport = RestfulProverProofTransport<Any, Any, DummyProofIndex>(
      restClient = restClient,
      vertx = Vertx.vertx(),
      chainId = 1,
      proofType = "execution",
      startBlockProvider = { it.startBlockNumber },
      endBlockProvider = { it.endBlockNumber },
      responseDtoClass = Any::class.java,
      pollingInterval = 1.seconds,
      pollingTimeout = 1.seconds,
      objectMapper = ObjectMapper(),
    )

    transport.removeRequests(42).get()

    val body = ObjectMapper().readTree(checkNotNull(restClient.lastPostBody))
    val criteria = body.get("criteria")
    assertThat(criteria.get("start_block_gte").asLong()).isEqualTo(42L)
    assertThat(criteria.get("proof_type").asText()).isEqualTo("execution")
  }

  private data class DummyProofIndex(
    override val startBlockNumber: ULong,
    override val endBlockNumber: ULong,
    override val startBlockTimestamp: Instant = Instant.fromEpochSeconds(0),
  ) : BlockInterval, ProofIndex

  private class RecordingHttpRestClient : HttpRestClient {
    var lastPostPath: String? = null
      private set
    var lastPostBody: String? = null
      private set

    override fun get(
      path: String,
      params: List<Pair<String, String>>,
      resultMapper: (Any?) -> Any?,
    ): SafeFuture<com.github.michaelbull.result.Result<Any?, ErrorResponse<RestErrorType>>> {
      return SafeFuture.failedFuture(UnsupportedOperationException("GET is not used in this test"))
    }

    override fun post(
      path: String,
      buffer: Buffer,
      resultMapper: (Any?) -> Any?,
    ): SafeFuture<com.github.michaelbull.result.Result<Any?, ErrorResponse<RestErrorType>>> {
      lastPostPath = path
      lastPostBody = buffer.toString()
      return SafeFuture.completedFuture(Ok(null))
    }
  }
}
