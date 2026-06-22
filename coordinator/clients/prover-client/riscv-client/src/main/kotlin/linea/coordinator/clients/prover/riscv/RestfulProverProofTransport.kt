package linea.coordinator.clients.prover.riscv

import com.fasterxml.jackson.annotation.JsonProperty
import com.fasterxml.jackson.databind.JsonNode
import com.fasterxml.jackson.databind.ObjectMapper
import com.github.michaelbull.result.Err
import com.github.michaelbull.result.Ok
import io.vertx.core.Vertx
import io.vertx.core.buffer.Buffer
import io.vertx.ext.web.client.HttpResponse
import linea.clients.ProverProofTransport
import linea.coordinator.clients.prover.serialization.JsonSerialization
import linea.domain.ProofIndex
import net.consensys.linea.async.AsyncRetryer
import net.consensys.linea.httprest.client.HttpRestClient
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Duration

/**
 * RESTful [ProverProofTransport].
 *
 * A proof job is addressed by `proof_type`, `start_block` and `end_block`, derived from the proof index. The
 * remote prover service exposes:
 *  - `GET  /v1/jobs/{proof_type}/{start_block}/{end_block}` — returns the job (status + optional proof_response);
 *  - `POST /v1/jobs/{proof_type}/{start_block}/{end_block}` — creates a job from `{ "proof_request": <requestDto> }`.
 *
 * @param proofType the `proof_type` path segment for this transport (e.g. "execution", "rollup", "rollup-aggregation").
 * @param startBlockProvider extracts the `start_block` path segment from a proof index.
 * @param endBlockProvider extracts the `end_block` path segment from a proof index.
 * @param responseDtoClass concrete [ResponseDto] type the `proof_response` payload is parsed into.
 */
class RestfulProverProofTransport<RequestDto : Any, ResponseDto, TProofIndex : ProofIndex>(
  private val restClient: HttpRestClient,
  private val vertx: Vertx,
  private val proofType: String,
  private val startBlockProvider: (TProofIndex) -> ULong,
  private val endBlockProvider: (TProofIndex) -> ULong,
  private val jobPathProvider: (TProofIndex) -> String = { proofIndex: TProofIndex ->
    "/v1/jobs/$proofType/${startBlockProvider(proofIndex)}/${endBlockProvider(proofIndex)}"
  },
  private val responseDtoClass: Class<ResponseDto>,
  private val pollingInterval: Duration,
  private val pollingTimeout: Duration,
  private val objectMapper: ObjectMapper = JsonSerialization.proofResponseMapperV1,
  private val log: Logger = LogManager.getLogger(RestfulProverProofTransport::class.java),
) : ProverProofTransport<RequestDto, ResponseDto, TProofIndex> {

  override fun isRequestAlreadySubmitted(proofIndex: TProofIndex): SafeFuture<Boolean> {
    return fetchJob(proofIndex).thenApply { job ->
      job != null && job.status in ACTIVE_JOB_STATUSES
    }
  }

  override fun submitRequest(proofIndex: TProofIndex, requestDto: RequestDto): SafeFuture<Unit> {
    val path = jobPathProvider(proofIndex)
    val body = SubmitJobRequest(proofRequest = objectMapper.valueToTree(requestDto))
    val buffer = Buffer.buffer(objectMapper.writeValueAsBytes(body))
    log.debug("Submitting proof request. POST {}", path)
    return restClient.post(path, buffer).thenApply { result ->
      when (result) {
        is Ok -> Unit
        is Err -> throw RuntimeException(
          "Failed to submit proof request: path=$path error=${result.error.type} message=${result.error.message}",
        )
      }
    }
  }

  override fun findResponse(proofIndex: TProofIndex): SafeFuture<ResponseDto?> {
    return fetchJob(proofIndex).thenApply { job -> job?.provedResponseOrNull() }
  }

  override fun awaitResponse(proofIndex: TProofIndex): SafeFuture<ResponseDto> {
    return AsyncRetryer.retry(
      vertx = vertx,
      backoffDelay = pollingInterval,
      timeout = pollingTimeout,
      stopRetriesPredicate = { responseDto -> responseDto != null },
      action = { findResponse(proofIndex) },
    ).thenApply { responseDto ->
      responseDto ?: throw RuntimeException("Timeout waiting for proof response. job=${jobPathProvider(proofIndex)}")
    }
  }

  /**
   * `GET`s the job. Returns the parsed job on a 2xx response, or null when the job is not available yet (e.g. a 404
   * before it is created, or any non-success status), so callers can treat "not found" as "not ready".
   */
  private fun fetchJob(proofIndex: TProofIndex): SafeFuture<ProverJobResponse?> {
    val path = jobPathProvider(proofIndex)
    return restClient.get(path).thenApply { result ->
      when (result) {
        is Ok -> {
          @Suppress("UNCHECKED_CAST")
          val response = result.value as HttpResponse<Buffer>
          objectMapper.readValue(response.bodyAsString(), ProverJobResponse::class.java)
        }

        is Err -> {
          log.trace("Proof job not available. path={} error={}", path, result.error.type)
          null
        }
      }
    }
  }

  private fun ProverJobResponse.provedResponseOrNull(): ResponseDto? {
    val proofResponse = this.proofResponse
    return if (status == STATUS_PROVED && proofResponse != null && !proofResponse.isNull && !proofResponse.isEmpty) {
      objectMapper.treeToValue(proofResponse, responseDtoClass)
    } else {
      null
    }
  }

  /** Body of `POST /v1/jobs/...`: the request DTO wrapped under a `proof_request` field. */
  private data class SubmitJobRequest(
    @get:JsonProperty("proof_request")
    val proofRequest: JsonNode,
  )

  /** Subset of the `GET /v1/jobs/...` response body this transport relies on. */
  private data class ProverJobResponse(
    @JsonProperty("status")
    val status: String,
    @JsonProperty("proof_response")
    val proofResponse: JsonNode? = null,
  )

  companion object {
    private const val STATUS_PENDING = "pending"
    private const val STATUS_CLAIMED = "claimed"
    private const val STATUS_PROVED = "proved"

    /** Statuses indicating a job already exists for a proof index (so a new request must not be submitted). */
    private val ACTIVE_JOB_STATUSES = setOf(STATUS_PENDING, STATUS_CLAIMED, STATUS_PROVED)
  }
}
