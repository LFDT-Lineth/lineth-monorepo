package linea.clients

import linea.domain.ProofIndex
import tech.pegasys.teku.infrastructure.async.SafeFuture

/**
 * Transport abstraction used by the generic RISC-V prover client to decouple the prover-client logic from the
 * mechanism used to submit a proof request and to obtain its response.
 *
 * The concrete strategy is file-based (`FileBasedProverProofTransport`): the request DTO is written to a JSON file
 * and the response is read back from a JSON file produced by the prover.
 *
 * @param RequestDto the serializable request payload produced by the client's request mapper.
 * @param ResponseDto the deserialized response payload understood by the client's response mapper.
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
   * Returns true when a response for [proofIndex] has already been existed (e.g. the response
   * file already exists, or a job for it is already known as proven to the remote service).
   */
  fun isResponseAlreadyExisted(proofIndex: TProofIndex): SafeFuture<Boolean>

  /**
   * Submits the [requestDto] for [proofIndex] by writing the JSON request file. Implementations should be idempotent.
   */
  fun submitRequest(proofIndex: TProofIndex, requestDto: RequestDto): SafeFuture<Unit>

  /**
   * Removes the submitted proof requests by removing their JSON request files. Implementations should be idempotent.
   */
  fun removeRequests(startBlockNumberGte: Long?): SafeFuture<Unit>

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
