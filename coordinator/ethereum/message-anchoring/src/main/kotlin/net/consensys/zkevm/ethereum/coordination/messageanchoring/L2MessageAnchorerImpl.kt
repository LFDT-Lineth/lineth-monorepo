package net.consensys.zkevm.ethereum.coordination.messageanchoring

import io.vertx.core.Vertx
import linea.kotlin.toBigInteger
import net.consensys.linea.async.AsyncRetryer
import net.consensys.linea.async.toSafeFuture
import net.consensys.linea.contract.L2MessageService
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.web3j.protocol.Web3j
import org.web3j.protocol.core.methods.response.EthBlock
import org.web3j.protocol.core.methods.response.TransactionReceipt
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.math.BigInteger
import java.util.concurrent.Callable
import kotlin.time.Duration

class L2MessageAnchorerImpl(
  private val vertx: Vertx,
  private val l2Web3j: Web3j,
  private val l2Client: L2MessageService,
  private val config: Config
) : L2MessageAnchorer {

  class Config(
    val receiptPollingInterval: Duration,
    val maxReceiptRetries: UInt,
    val blocksToFinalisation: Long
  )

  private val log: Logger = LogManager.getLogger(this::class.java)

  override fun anchorMessages(
    sendMessageEvents: List<SendMessageEvent>,
    finalRollingHash: ByteArray
  ): SafeFuture<TransactionReceipt> {
    log.debug(
      "Anchoring using rolling hash {}, hashes={}",
      sendMessageEvents.count(),
      sendMessageEvents.map { it.messageHash.toHexString() }
    )

    return vertx.executeBlocking(
      Callable {
        l2Client.anchorL1L2MessageHashes(
          sendMessageEvents.map { arr -> arr.messageHash.toArray() },
          sendMessageEvents.first().messageNumber.toBigInteger(),
          sendMessageEvents.last().messageNumber.toBigInteger(),
          finalRollingHash
        )
      },
      true
    ).toSafeFuture().thenCompose { anchorMessageHashesCall ->
      anchorMessageHashesCall.sendAsync()
        .thenCompose { txReceipt ->
          val safeBlock = txReceipt.blockNumber.add(
            BigInteger.valueOf(config.blocksToFinalisation)
          )
          AsyncRetryer.retry(
            vertx,
            maxRetries = config.maxReceiptRetries.toInt(),
            backoffDelay = config.receiptPollingInterval,
            stopRetriesPredicate = transactionIsSafe(safeBlock)
          ) {
            SafeFuture.of(l2Web3j.ethGetBlockByNumber({ "latest" }, false).sendAsync())
          }.exceptionally { _ -> null }
            .thenApply {
              txReceipt
            }
        }
    }
  }

  private fun transactionIsSafe(safeBlockNumber: BigInteger) =
    { result: EthBlock -> result.block.number >= safeBlockNumber }
}
