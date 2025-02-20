package net.consensys.linea.contract

import io.vertx.core.Vertx
import linea.kotlin.toULong
import net.consensys.linea.async.toSafeFuture
import net.consensys.zkevm.coordinator.clients.L2MessageServiceClient
import net.consensys.zkevm.coordinator.clients.L2MessageServiceLogsClient
import net.consensys.zkevm.domain.L2RollingHashUpdatedEvent
import org.web3j.protocol.core.DefaultBlockParameter
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.Callable

class Web3JL2MessageService(
  private val vertx: Vertx,
  private val l2MessageServiceLogsClient: L2MessageServiceLogsClient,
  private val web3jL2MessageService: L2MessageService
) : L2MessageServiceClient, L2MessageServiceLogsClient by l2MessageServiceLogsClient {
  val EMPTY_VALUE_ERROR_MESSAGE = "Empty value (0x) returned from contract"

  @Synchronized
  override fun getLastAnchoredMessageUpToBlock(blockNumberInclusive: Long): SafeFuture<L2RollingHashUpdatedEvent> {
    return vertx.executeBlocking(
      Callable {
        web3jL2MessageService
          .setDefaultBlockParameter(DefaultBlockParameter.valueOf(blockNumberInclusive.toBigInteger()))

        lastAnchoredL1MessageNumber()
          .thenCompose { lastAnchoredL1MessageNumber ->
            if (lastAnchoredL1MessageNumber == 0UL) {
              // there won't rolling hash for message number 0, default empty rolling hash
              SafeFuture.completedFuture(L2RollingHashUpdatedEvent(0UL, ByteArray(32)))
            } else {
              findRollingHash(lastAnchoredL1MessageNumber.toLong())
                .thenApply { rollingHash ->
                  L2RollingHashUpdatedEvent(
                    messageNumber = lastAnchoredL1MessageNumber,
                    messageRollingHash = rollingHash
                  )
                }
            }
          }.get()
      },
      /*ordered*/true
    ).toSafeFuture()
  }

  private fun findRollingHash(messageNumber: Long): SafeFuture<ByteArray> {
    return SafeFuture.of(
      web3jL2MessageService
        .l1RollingHashes(messageNumber.toBigInteger()).sendAsync()
    )
  }

  private fun lastAnchoredL1MessageNumber(): SafeFuture<ULong> {
    return SafeFuture.of(web3jL2MessageService.lastAnchoredL1MessageNumber().sendAsync())
      .thenApply { it.toULong() }
      .exceptionallyCompose {
        // Web3J has a bug and cannot decode 0 value with result as `0x`
        if (it.message?.contains(EMPTY_VALUE_ERROR_MESSAGE) == true) {
          SafeFuture.completedFuture(0UL)
        } else {
          SafeFuture.failedFuture(it)
        }
      }
  }
}
