package linea.coordinator.clients.prover.riscv

import linea.domain.ProofIndex
import tech.pegasys.teku.infrastructure.async.SafeFuture

/**
 * Transport abstraction used by [GenericRiscVProverClient] to decouple the prover-client logic from the
 * mechanism used to submit a proof request and to obtain its response.
 *
 * Two concrete strategies are envisaged:
 *  - a file-based one ([FileBasedProverProofTransport]): the request DTO is written to a JSON file and the
 *    response is read back from a JSON file produced by the prover;
 *  - a RESTful one ([RestfulProverProofTransport]): the request DTO is sent as the body of an HTTP POST and the
 *    response is polled via periodic HTTP GET calls.
 *
 * The exact REST call syntax (endpoints, payload envelope, status semantics) is not finalized yet, hence the
 * RESTful implementation is currently a skeleton.
 *
 * @param RequestDto the serializable request payload produced by the client's request mapper.
 * @param ResponseDto the deserialized response payload understood by the client's response mapper. For proof types
 *   whose response is not parsed (e.g. execution), this can be [Any] and the value is only used as an availability
 *   signal.
 * @param TProofIndex the proof index uniquely identifying a request/response pair.
 */
interface ProverProofTransport<RequestDto : Any, ResponseDto, TProofIndex : ProofIndex> {

  /**
   * Returns true when a request for [proofIndex] has already been submitted through this transport (e.g. the request
   * file already exists, or a job for it is already known to the remote service), so it does not need to be
   * re-submitted. Used to keep [submitRequest] idempotent.
   */
  fun isRequestAlreadySubmitted(proofIndex: TProofIndex): SafeFuture<Boolean>

  /**
   * Submits the [requestDto] for [proofIndex]. For the file-based transport this writes the JSON request file; for the
   * RESTful transport this issues the POST call. Implementations should be idempotent.
   */
  fun submitRequest(proofIndex: TProofIndex, requestDto: RequestDto): SafeFuture<Unit>

  /**
   * Returns the response for [proofIndex] if it is already available, otherwise null. Does not block waiting for the
   * response to be produced.
   */
  fun findResponse(proofIndex: TProofIndex): SafeFuture<ResponseDto?>

  /**
   * Waits/polls until the response for [proofIndex] becomes available and returns it, failing the future on timeout.
   */
  fun awaitResponse(proofIndex: TProofIndex): SafeFuture<ResponseDto>
}
