package net.consensys.zkevm.ethereum.coordination.aggregation

import net.consensys.zkevm.domain.Aggregation
import net.consensys.zkevm.domain.AggregationProofIndex
import net.consensys.zkevm.persistence.AggregationsRepository
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.function.Consumer
import java.util.function.Supplier

fun interface AggregationProofHandler {
  fun acceptNewAggregation(provenAggregation: Aggregation): SafeFuture<*>
}

fun interface AggregationProofRequestHandler {
  fun acceptNewAggregationProofRequest(
    proofIndex: AggregationProofIndex,
    unProvenAggregation: Aggregation,
  ): SafeFuture<*>
}

class AggregationProofHandlerImpl(
  private val aggregationsRepository: AggregationsRepository,
  private val provenAggregationEndBlockNumberConsumer: Consumer<ULong>,
  private val provenConsecutiveAggregationEndBlockNumberConsumer: Consumer<ULong>,
  private val lastFinalizedBlockNumberSupplier: Supplier<ULong>,
  private val log: Logger = LogManager.getLogger(AggregationProofHandlerImpl::class.java),
) : AggregationProofHandler {
  override fun acceptNewAggregation(provenAggregation: Aggregation): SafeFuture<*> {
    check(provenAggregation.aggregationProof != null) {
      "Aggregation proof is expected to be not null for proven aggregation: " +
        "aggregation=${provenAggregation.intervalString()}"
    }
    return aggregationsRepository
      .saveNewAggregation(aggregation = provenAggregation)
      .thenPeek {
        provenAggregationEndBlockNumberConsumer.accept(provenAggregation.endBlockNumber)
      }
      .whenException {
        log.error(
          "Error saving proven aggregation to DB: aggregation={} errorMessage={}",
          provenAggregation.intervalString(),
          it.message,
          it,
        )
      }
      .thenPeek {
        aggregationsRepository.findHighestConsecutiveEndBlockNumber(
          lastFinalizedBlockNumberSupplier.get().toLong() + 1L,
        )
          .thenApply { it ->
            if (it != null) {
              provenConsecutiveAggregationEndBlockNumberConsumer.accept(it.toULong())
            }
          }
          .whenException {
            log.warn(
              "Failed to get consecutive aggregation end block number from DB: aggregation={} errorMessage={}",
              provenAggregation.intervalString(),
              it.message,
              it,
            )
          }
      }
  }
}
