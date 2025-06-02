package net.consensys.zkevm.ethereum.coordination.blockcreation

import com.github.michaelbull.result.Err
import com.github.michaelbull.result.Ok
import linea.domain.BlockNumberAndHash
import linea.kotlin.ByteArrayExt
import net.consensys.linea.errors.ErrorResponse
import net.consensys.zkevm.coordinator.clients.RollupForkChoiceUpdatedClient
import net.consensys.zkevm.coordinator.clients.RollupForkChoiceUpdatedError
import net.consensys.zkevm.coordinator.clients.RollupForkChoiceUpdatedResponse
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import org.mockito.kotlin.any
import org.mockito.kotlin.mock
import org.mockito.kotlin.verify
import org.mockito.kotlin.whenever
import tech.pegasys.teku.infrastructure.async.SafeFuture

class ForkChoiceUpdaterImplTest {

  @Test
  fun dispatchFinalizedBlockNotification_allClientsSuccess() {
    val mockClient1 = mock<RollupForkChoiceUpdatedClient>()
    whenever(
      mockClient1.rollupForkChoiceUpdated(any()),
    )
      .thenReturn(SafeFuture.completedFuture(Ok(RollupForkChoiceUpdatedResponse("success"))))
    val mockClient2 = mock<RollupForkChoiceUpdatedClient>()
    whenever(
      mockClient2.rollupForkChoiceUpdated(any()),
    )
      .thenReturn(SafeFuture.completedFuture(Ok(RollupForkChoiceUpdatedResponse("success"))))

    val finalizedBlockNotifierImpl = ForkChoiceUpdaterImpl(listOf(mockClient1, mockClient2))
    val blockNumberAndHash = BlockNumberAndHash(100U, ByteArrayExt.random32())
    val result = finalizedBlockNotifierImpl.updateFinalizedBlock(blockNumberAndHash)
    assertThat(result).isCompleted()
    verify(mockClient1).rollupForkChoiceUpdated(blockNumberAndHash)
    verify(mockClient2).rollupForkChoiceUpdated(blockNumberAndHash)
  }

  @Test
  fun dispatchFinalizedBlockNotification_someClientsFail() {
    val mockClient1 = mock<RollupForkChoiceUpdatedClient>()
    whenever(
      mockClient1.rollupForkChoiceUpdated(any()),
    )
      .thenReturn(SafeFuture.completedFuture(Ok(RollupForkChoiceUpdatedResponse("success"))))
    val mockClient2 = mock<RollupForkChoiceUpdatedClient>()
    whenever(
      mockClient2.rollupForkChoiceUpdated(any()),
    )
      .thenReturn(SafeFuture.completedFuture(Err(ErrorResponse(RollupForkChoiceUpdatedError.UNKNOWN, ""))))

    val finalizedBlockNotifierImpl = ForkChoiceUpdaterImpl(listOf(mockClient1, mockClient2))
    val blockNumberAndHash = BlockNumberAndHash(100U, ByteArrayExt.random32())
    val result = finalizedBlockNotifierImpl.updateFinalizedBlock(blockNumberAndHash)
    assertThat(result).isCompleted()
    verify(mockClient1).rollupForkChoiceUpdated(blockNumberAndHash)
    verify(mockClient2).rollupForkChoiceUpdated(blockNumberAndHash)
  }
}
