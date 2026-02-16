package linea.contract

import io.vertx.core.Vertx
import linea.contract.l1.ContractVersionAwaiter
import linea.contract.l1.ContractVersionProvider
import linea.contract.l1.LineaRollupContractVersion
import linea.domain.BlockParameter
import net.consensys.linea.async.AsyncRetryer
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Duration
import kotlin.time.Duration.Companion.seconds

class Web3JContractVersionAwaiter<VersionType : Comparable<VersionType>>(
  val vertx: Vertx,
  val versionProvider: ContractVersionProvider<VersionType>,
  val log: Logger = LogManager.getLogger(Web3JContractVersionAwaiter::class.java.name),
) : ContractVersionAwaiter<VersionType> {
  override fun awaitVersion(
    minTargetVersion: VersionType,
    highestBlockTag: BlockParameter,
    timeout: Duration?,
  ): SafeFuture<VersionType> {
    return versionProvider
      .getVersion(highestBlockTag)
      .thenCompose { contractVersion ->
        if (contractVersion >= minTargetVersion) {
          log.info(
            "contract version reached: minTargetVersion={}, currentVersion={}",
            minTargetVersion,
            contractVersion,
          )
          SafeFuture.completedFuture(contractVersion)
        } else {
          log.info(
            "waiting for contract version: minTargetVersion={}, currentVersion={}",
            minTargetVersion,
            contractVersion,
          )

          AsyncRetryer.retry(
            vertx = vertx,
            backoffDelay = 1.seconds,
            timeout = timeout,
            stopRetriesPredicate = { contractVersion ->
              contractVersion >= minTargetVersion
            },
            action = { versionProvider.getVersion(highestBlockTag) },
          )
        }
      }
  }

  companion object {
    fun lineaRollupVersionWaiter(
      vertx: Vertx,
      lineaVersionProvider: ContractVersionProvider<LineaRollupContractVersion>,
    ): Web3JContractVersionAwaiter<LineaRollupContractVersion> =
      Web3JContractVersionAwaiter(vertx, lineaVersionProvider)
  }
}
