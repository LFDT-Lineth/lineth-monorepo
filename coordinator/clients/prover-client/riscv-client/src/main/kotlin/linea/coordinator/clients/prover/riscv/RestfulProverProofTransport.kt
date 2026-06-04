package linea.coordinator.clients.prover.riscv

import linea.domain.ProofIndex
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Duration

/**
 * RESTful [ProverProofTransport] (SKELETON).
 *
 * Intended behaviour once the prover REST contract is finalized:
 *  - [submitRequest] sends the whole request DTO as the body of an HTTP POST to a "create proof" endpoint;
 *  - [findResponse] performs a single HTTP GET against a "get proof" endpoint and returns the parsed response if the
 *    proof is ready (e.g. HTTP 200), or null while it is still being produced (e.g. HTTP 404 / 202);
 *  - [awaitResponse] polls that GET endpoint on [pollingInterval] until the proof is ready or [pollingTimeout] elapses;
 *  - [isRequestAlreadySubmitted] queries the service for an existing job for the proof index.
 *
 * The exact endpoints, request/response envelopes and status semantics are NOT finalized yet, so the methods below are
 * left as TODOs. Wire an HTTP client (e.g. vertx WebClient) and the proof-index → URL mapping here when the contract
 * lands.
 */
class RestfulProverProofTransport<RequestDto : Any, ResponseDto, TProofIndex : ProofIndex>(
  private val baseUrl: String,
  private val pollingInterval: Duration,
  private val pollingTimeout: Duration,
  private val log: Logger = LogManager.getLogger(RestfulProverProofTransport::class.java),
) : ProverProofTransport<RequestDto, ResponseDto, TProofIndex> {

  override fun isRequestAlreadySubmitted(proofIndex: TProofIndex): SafeFuture<Boolean> {
    // TODO: GET <baseUrl>/<job-id-for(proofIndex)> and map an existing job to `true`.
    TODO("Define the REST 'job exists' call once the prover REST contract is finalized")
  }

  override fun submitRequest(proofIndex: TProofIndex, requestDto: RequestDto): SafeFuture<Unit> {
    // TODO: POST <baseUrl>/... with `requestDto` as the JSON body.
    TODO("Define the REST POST 'create proof' call once the prover REST contract is finalized")
  }

  override fun findResponse(proofIndex: TProofIndex): SafeFuture<ResponseDto?> {
    // TODO: single GET <baseUrl>/... -> parsed ResponseDto when ready, null while still proving.
    TODO("Define the REST GET 'get proof' call once the prover REST contract is finalized")
  }

  override fun awaitResponse(proofIndex: TProofIndex): SafeFuture<ResponseDto> {
    // TODO: poll findResponse() every `pollingInterval` until non-null or `pollingTimeout` elapses.
    TODO("Define the REST polling loop once the prover REST contract is finalized")
  }
}
