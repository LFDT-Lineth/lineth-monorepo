package linea.executionwitness

import com.github.michaelbull.result.Err
import com.github.michaelbull.result.Ok
import com.github.michaelbull.result.Result
import linea.domain.BlockParameter
import linea.error.ErrorResponse
import tech.pegasys.teku.infrastructure.async.SafeFuture

class FakeExecutionWitnessClient(
  private val witnessesByBlock: Map<BlockParameter, ExecutionWitness> = emptyMap(),
) : ExecutionWitnessClient {

  override fun getExecutionWitness(
    block: BlockParameter,
  ): SafeFuture<Result<ExecutionWitness, ErrorResponse<ExecutionWitnessError>>> {
    val witness = witnessesByBlock[block]
      ?: return SafeFuture.completedFuture(
        Err(
          ErrorResponse(ExecutionWitnessError.NULL_RESULT, "no witness configured for block=$block"),
        ),
      )
    return SafeFuture.completedFuture(Ok(witness))
  }
}
