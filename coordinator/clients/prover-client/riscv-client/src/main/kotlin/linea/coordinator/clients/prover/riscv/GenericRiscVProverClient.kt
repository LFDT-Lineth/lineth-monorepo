package linea.coordinator.clients.prover.riscv

import linea.clients.ProverProofRequestCreator
import linea.clients.ProverProofResponseChecker
import linea.domain.ProofIndex
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.atomic.AtomicLong
import java.util.function.Supplier

/**
 * Generic prover client for the RISC-V provers.
 *
 * It mirrors the responsibilities of `GenericFileBasedProverClient` (it implements both [ProverProofResponseChecker]
 * and [ProverProofRequestCreator] and exposes [requestProof]) but is NOT tied to a file-based approach: the
 * submit/find/await mechanics are delegated to an injected [ProverProofTransport]. The transport decides whether the
 * request and response travel through JSON files on disk or through RESTful calls to a remote prover service.
 *
 * The client itself only knows how to:
 *  - derive the [TProofIndex] from a domain request ([proofIndexProvider]);
 *  - map a domain request to a serializable request DTO ([requestMapper]);
 *  - map a response DTO back to the domain response ([parseResponse], by default delegating to [responseMapper]).
 *
 * @param Request domain proof request type.
 * @param Response domain proof response type.
 * @param RequestDto serializable request payload produced by [requestMapper].
 * @param ResponseDto response payload returned by the transport and consumed by [responseMapper].
 * @param TProofIndex proof index uniquely identifying a request/response pair.
 */
open class GenericRiscVProverClient<Request, Response, RequestDto, ResponseDto, TProofIndex>(
  private val transport: ProverProofTransport<RequestDto, ResponseDto, TProofIndex>,
  private val proofIndexProvider: (Request) -> TProofIndex,
  private val requestMapper: (Request) -> SafeFuture<RequestDto>,
  private val responseMapper: (TProofIndex, ResponseDto) -> Response,
  private val proofTypeLabel: String,
  private val log: Logger = LogManager.getLogger(GenericRiscVProverClient::class.java),
) : ProverProofResponseChecker<Response, TProofIndex>,
  ProverProofRequestCreator<Request, TProofIndex>,
  Supplier<Number>
  where TProofIndex : ProofIndex, Request : Any, RequestDto : Any {

  private val responsesWaiting = AtomicLong(0)
  override fun get(): Long = responsesWaiting.get()

  override fun findProofResponse(proofIndex: TProofIndex): SafeFuture<Response?> {
    log.trace("Checking if response is available. {}={}", proofTypeLabel, proofIndex)
    return transport.findResponse(proofIndex)
      .thenApply { responseDto -> responseDto?.let { parseResponse(it, proofIndex) } }
  }

  override fun createProofRequest(proofRequest: Request): SafeFuture<TProofIndex> {
    val proofIndex = proofIndexProvider(proofRequest)
    log.debug(
      "Creating proof: {}={}, proofIndexProvider={}",
      proofTypeLabel,
      proofIndex,
      proofIndexProvider.toString(),
    )
    return transport.isRequestAlreadySubmitted(proofIndex)
      .thenCompose { alreadySubmitted ->
        if (alreadySubmitted) {
          log.debug("request already submitted: {}={}", proofTypeLabel, proofIndex)
          SafeFuture.completedFuture(proofIndex)
        } else {
          requestMapper(proofRequest)
            .thenCompose { requestDto ->
              log.trace("Submitting proof request. {}={}", proofTypeLabel, proofIndex)
              transport.submitRequest(proofIndex, requestDto)
                .thenApply {
                  log.trace("Submitted proof request. {}={}", proofTypeLabel, proofIndex)
                  proofIndex
                }
            }
        }
      }
  }

  fun requestProof(proofRequest: Request): SafeFuture<Response> {
    val proofIndex = proofIndexProvider(proofRequest)
    log.debug(
      "Requesting proof: {}={}, proofIndexProvider={}",
      proofTypeLabel,
      proofIndex,
      proofIndexProvider.toString(),
    )

    return findProofResponse(proofIndex)
      .thenCompose { response ->
        if (response != null) {
          SafeFuture.completedFuture(response)
        } else {
          responsesWaiting.incrementAndGet()
          createProofRequest(proofRequest)
            .thenCompose { transport.awaitResponse(proofIndex) }
            .thenApply { responseDto ->
              responsesWaiting.decrementAndGet()
              parseResponse(responseDto, proofIndex)
            }
        }
      }
      .whenException {
        log.error(
          "Failed to get proof: {}={} errorMessage={}",
          proofTypeLabel,
          proofIndex,
          it.message,
          it,
        )
      }
  }

  /**
   * Parses a response DTO obtained from the transport into the domain response. Overridable for proof types whose
   * response is derived from the [proofIndex] rather than from the transport payload (e.g. execution).
   */
  protected open fun parseResponse(responseDto: ResponseDto, proofIndex: TProofIndex): Response {
    return responseMapper(proofIndex, responseDto)
  }
}
