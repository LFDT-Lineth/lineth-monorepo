package linea.web3j.ethapi

import linea.domain.Block
import linea.domain.BlockParameter
import linea.domain.BlockWithTxHashes
import linea.domain.EthLog
import linea.ethapi.EthApiClient
import linea.web3j.domain.toDomain
import linea.web3j.domain.toWeb3j
import linea.web3j.mapToDomainWithTxHashes
import linea.web3j.requestAsync
import linea.web3j.toDomain
import org.web3j.protocol.Web3j
import org.web3j.protocol.core.methods.request.EthFilter
import org.web3j.protocol.core.methods.response.Log
import tech.pegasys.teku.infrastructure.async.SafeFuture

/**
 * Web3j Adapter of EthApiClient
 * Request retries is responsibility of another class
 */
class Web3jEthApiClient(
  val web3jClient: Web3j
) : EthApiClient {

  override fun findBlockByNumber(blockParameter: BlockParameter): SafeFuture<Block?> {
    return web3jClient
      .ethGetBlockByNumber(blockParameter.toWeb3j(), true)
      .requestAsync { resp -> resp.block?.toDomain() }
  }

  override fun findBlockByNumberWithoutTransactionsData(
    blockParameter: BlockParameter
  ): SafeFuture<BlockWithTxHashes?> {
    return web3jClient
      .ethGetBlockByNumber(blockParameter.toWeb3j(), false)
      .requestAsync { resp -> resp.block?.let(::mapToDomainWithTxHashes) }
  }

  override fun getLogs(
    fromBlock: BlockParameter,
    toBlock: BlockParameter,
    address: String,
    topics: List<String?>
  ): SafeFuture<List<EthLog>> {
    val ethFilter = EthFilter(
      /*fromBlock*/ fromBlock.toWeb3j(),
      /*toBlock*/ toBlock.toWeb3j(),
      /*address*/ address
    ).apply {
      topics.forEach { addSingleTopic(it) }
    }

    return web3jClient
      .ethGetLogs(ethFilter)
      .requestAsync { logsResponse ->
        if (logsResponse.logs != null) {
          @Suppress("UNCHECKED_CAST")
          (logsResponse.logs as List<org.web3j.protocol.core.methods.response.EthLog.LogResult<Log>>)
            .map { logResult -> logResult.get().toDomain() }
        } else {
          emptyList()
        }
      }
  }
}
