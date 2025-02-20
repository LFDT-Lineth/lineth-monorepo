@file:Suppress("DEPRECATION")

package net.consensys.zkevm.ethereum.submission

import build.linea.web3j.Web3JLogsClient
import linea.kotlin.toULong
import net.consensys.linea.contract.L2MessageService
import net.consensys.zkevm.ethereum.coordination.conflation.upgrade.SwitchProvider
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.web3j.abi.EventEncoder
import org.web3j.abi.TypeEncoder
import org.web3j.protocol.core.DefaultBlockParameter
import org.web3j.protocol.core.DefaultBlockParameterName
import org.web3j.protocol.core.methods.request.EthFilter
import org.web3j.protocol.core.methods.response.Log
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.math.BigInteger
import java.util.concurrent.ConcurrentHashMap
import org.web3j.abi.datatypes.Uint as Web3jUInt

@Suppress("DEPRECATION")
class SwitchProviderImpl(
  private val web3jLogsClient: Web3JLogsClient,
  private val l2MessageService: L2MessageService,
  earliestBlock: ULong
) : SwitchProvider {
  private val log: Logger = LogManager.getLogger(this::class.java)
  private val earliestBlockDefaultBlockParameter =
    DefaultBlockParameter.valueOf(BigInteger.valueOf(earliestBlock.toLong()))

  private var blockCache: MutableMap<SwitchProvider.ProtocolSwitches, ULong> = ConcurrentHashMap()

  override fun getSwitch(version: SwitchProvider.ProtocolSwitches): SafeFuture<ULong?> {
    return if (blockCache.containsKey(version)) {
      SafeFuture.completedFuture(blockCache[version])
    } else {
      lookForEvent(version)
    }
  }

  private fun lookForEvent(
    version: SwitchProvider.ProtocolSwitches
  ): SafeFuture<ULong?> {
    return findServiceVersionMigratedEventBlockNumber(version)
      .thenCompose { blockNumber ->
        if (blockNumber != null) {
          blockCache[version] = blockNumber
          SafeFuture.completedFuture(blockNumber)
        } else {
          SafeFuture.completedFuture(null)
        }
      }
  }

  private fun findServiceVersionMigratedEventBlockNumber(
    version: SwitchProvider.ProtocolSwitches
  ): SafeFuture<ULong> {
    val ethFilter =
      EthFilter(
        earliestBlockDefaultBlockParameter,
        DefaultBlockParameterName.LATEST,
        l2MessageService.contractAddress
      )
    ethFilter.addSingleTopic(EventEncoder.encode(L2MessageService.SERVICEVERSIONMIGRATED_EVENT))
    ethFilter.addSingleTopic("0x" + TypeEncoder.encode(Web3jUInt(BigInteger.valueOf(version.int.toLong()))))

    return web3jLogsClient.getLogs(ethFilter)
      .thenApply { logs ->
        if (logs.isNullOrEmpty()) {
          null
        } else {
          val log: Log = logs.last()
          log.blockNumber.toULong()
        }
      }
  }
}
