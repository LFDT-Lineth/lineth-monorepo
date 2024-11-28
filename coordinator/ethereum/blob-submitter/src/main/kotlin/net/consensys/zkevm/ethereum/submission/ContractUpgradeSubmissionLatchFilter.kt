package net.consensys.zkevm.ethereum.submission

import build.linea.contract.l1.ContractVersionProvider
import build.linea.contract.l1.LineaContractVersion
import build.linea.domain.BlockInterval
import net.consensys.linea.async.AsyncFilter
import org.apache.logging.log4j.LogManager
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.atomic.AtomicBoolean

class ContractUpgradeSubmissionLatchFilter<T : BlockInterval>(
  private val l2SwitchBlockNumber: ULong? = null,
  private val contractVersionProvider: ContractVersionProvider<LineaContractVersion>,
  private val currentContractVersion: LineaContractVersion,
  private val expectedNewContractVersion: LineaContractVersion
) : AsyncFilter<T> {
  private val log = LogManager.getLogger(this::class.java)
  private val latchEnabled = AtomicBoolean(false)

  override fun invoke(items: List<T>): SafeFuture<List<T>> {
    if (l2SwitchBlockNumber == null || items.isEmpty()) {
      return SafeFuture.completedFuture(items)
    }

    return contractVersionProvider
      .getVersion()
      .thenApply { contractVersion ->
        if (contractVersion == currentContractVersion) {
          val blobs = items.filter { it.startBlockNumber <= l2SwitchBlockNumber }
          if (blobs.isEmpty()) {
            latchEnabled.set(true)
            log.info(
              "Contract upgrade latch blocked submission: " +
                "l2SwitchBlockNumber={} nextBlobToSubmit={} contractVersion={} waiting newContractVersion={}",
              l2SwitchBlockNumber,
              items.firstOrNull()?.intervalString(),
              contractVersion,
              expectedNewContractVersion
            )
          }
          blobs
        } else {
          if (latchEnabled.get()) {
            latchEnabled.set(false)
            log.info(
              "Contract upgrade latch blocked submission: " +
                "l2SwitchBlockNumber={} nextBlobToSubmit={} newContractVersion={} expectedNewContractVersion={}",
              l2SwitchBlockNumber,
              items.firstOrNull()?.intervalString(),
              contractVersion,
              expectedNewContractVersion
            )
          }
          items
        }
      }
  }
}
