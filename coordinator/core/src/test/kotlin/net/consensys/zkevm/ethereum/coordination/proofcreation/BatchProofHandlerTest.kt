package net.consensys.zkevm.ethereum.coordination.proofcreation

import io.vertx.junit5.Timeout
import io.vertx.junit5.VertxExtension
import io.vertx.junit5.VertxTestContext
import linea.domain.Batch
import linea.persistence.BatchesRepository
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import org.mockito.kotlin.eq
import org.mockito.kotlin.mock
import org.mockito.kotlin.times
import org.mockito.kotlin.verify
import org.mockito.kotlin.whenever
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.TimeUnit

@ExtendWith(VertxExtension::class)
class BatchProofHandlerTest {
  @Test
  @Timeout(1, timeUnit = TimeUnit.SECONDS)
  fun acceptNewBatch_addsABatchToTheRepository(testContext: VertxTestContext) {
    val batchesRepository = mock<BatchesRepository>()
    val batchProofHandler = BatchProofHandlerImpl(batchesRepository)

    val batch = Batch(1UL, 2UL)
    whenever(batchesRepository.saveNewBatch(eq(batch))).thenReturn(SafeFuture.completedFuture(Unit))
    batchProofHandler
      .acceptNewBatch(batch)
      .thenApply {
        testContext.verify { verify(batchesRepository, times(1)).saveNewBatch(eq(batch)) }
        testContext.completeNow()
      }
      .whenException(testContext::failNow)
  }
}
