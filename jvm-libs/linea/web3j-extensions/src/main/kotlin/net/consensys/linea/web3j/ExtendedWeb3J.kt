package net.consensys.linea.web3j

import build.linea.web3j.domain.toWeb3j
import linea.domain.Block
import linea.web3j.toDomain
import net.consensys.linea.BlockParameter
import net.consensys.linea.async.toSafeFuture
import org.web3j.protocol.Web3j
import org.web3j.protocol.core.DefaultBlockParameter
import org.web3j.protocol.core.Response
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.math.BigInteger

/**
 * Web3J is quite complex to Mock and Unit test, this is just a tinny interface to make
 * BlockCreationMonitor more testable
 */
interface ExtendedWeb3J {
  val web3jClient: Web3j
  fun ethBlockNumber(): SafeFuture<BigInteger>
  fun ethGetBlock(blockParameter: BlockParameter): SafeFuture<Block?>
  fun ethGetBlockTimestampByNumber(blockNumber: Long): SafeFuture<BigInteger>
}

class ExtendedWeb3JImpl(override val web3jClient: Web3j) : ExtendedWeb3J {

  private fun buildException(error: Response.Error): Exception =
    RuntimeException("${error.code}: ${error.message}")

  override fun ethBlockNumber(): SafeFuture<BigInteger> {
    return SafeFuture.of(web3jClient.ethBlockNumber().sendAsync()).thenCompose { response ->
      if (response.hasError()) {
        SafeFuture.failedFuture(buildException(response.error))
      } else {
        SafeFuture.completedFuture(response.blockNumber)
      }
    }
  }

  override fun ethGetBlock(blockParameter: BlockParameter): SafeFuture<Block?> {
    return web3jClient
      .ethGetBlockByNumber(
        blockParameter.toWeb3j(),
        true
      )
      .sendAsync()
      .toSafeFuture()
      .thenCompose { response ->
        if (response.hasError()) {
          SafeFuture.failedFuture(buildException(response.error))
        } else {
          response.block?.let {
            SafeFuture.completedFuture(response.block.toDomain())
          } ?: SafeFuture.failedFuture(RuntimeException("Block $blockParameter not found!"))
        }
      }
  }

  override fun ethGetBlockTimestampByNumber(
    blockNumber: Long
  ): SafeFuture<BigInteger> {
    return SafeFuture.of(
      web3jClient
        .ethGetBlockByNumber(
          DefaultBlockParameter.valueOf(BigInteger.valueOf(blockNumber)),
          false
        )
        .sendAsync()
    )
      .thenCompose { response ->
        if (response.hasError()) {
          SafeFuture.failedFuture(buildException(response.error))
        } else {
          response.block?.let {
            SafeFuture.completedFuture(response.block.timestamp)
          } ?: SafeFuture.failedFuture(Exception("Block $blockNumber not found!"))
        }
      }
  }
}
