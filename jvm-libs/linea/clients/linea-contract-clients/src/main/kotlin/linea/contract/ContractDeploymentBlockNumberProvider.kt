package linea.contract

import linea.contract.events.Upgraded
import linea.domain.BlockParameter
import linea.ethapi.EthApiClient
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.atomic.AtomicReference

typealias ContractDeploymentBlockNumberProvider = () -> SafeFuture<ULong>

class StaticContractDeploymentBlockNumberProvider(
  private val deploymentBlockNumber: ULong
) : ContractDeploymentBlockNumberProvider {
  override fun invoke(): SafeFuture<ULong> {
    return SafeFuture.completedFuture(deploymentBlockNumber)
  }
}

class EventBasedContractDeploymentBlockNumberProvider(
  private val ethApiClient: EthApiClient,
  private val contractAddress: String,
  private val log: Logger = LogManager.getLogger(EventBasedContractDeploymentBlockNumberProvider::class.java)
) : ContractDeploymentBlockNumberProvider {
  private val deploymentBlockNumberCache = AtomicReference<ULong>(0UL)

  fun getDeploymentBlock(): SafeFuture<ULong> {
    if (deploymentBlockNumberCache.get() != 0UL) {
      return SafeFuture.completedFuture(deploymentBlockNumberCache.get())
    } else {
      return ethApiClient
        .getLogs(
          fromBlock = BlockParameter.Tag.EARLIEST,
          toBlock = BlockParameter.Tag.LATEST,
          address = contractAddress,
          topics = listOf(Upgraded.topic)
        ).thenApply { logs ->
          if (logs.isEmpty()) {
            throw IllegalStateException("Upgraded event not found: contractAddress=$contractAddress")
          }
          val blockNumber = logs.minByOrNull { it.blockNumber }!!.blockNumber
          deploymentBlockNumberCache.set(blockNumber)
          blockNumber
        }
        .whenException {
          log.error(
            "Failed to get deployment block number for contract={} errorMessage=",
            contractAddress,
            it.message
          )
        }
    }
  }

  override fun invoke(): SafeFuture<ULong> {
    return getDeploymentBlock()
  }
}
