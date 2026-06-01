package linea.executionwitness

import com.github.michaelbull.result.Result
import linea.domain.BlockParameter
import linea.error.ErrorResponse
import tech.pegasys.teku.infrastructure.async.SafeFuture

interface ExecutionWitnessClient {
  fun getExecutionWitness(
    block: BlockParameter,
  ): SafeFuture<Result<ExecutionWitness, ErrorResponse<ExecutionWitnessError>>>
}
