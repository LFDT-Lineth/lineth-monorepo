package linea.consensus

import io.vertx.core.Vertx
import net.consensys.linea.LineaBesuEngineBlockTagUpdater
import net.consensys.linea.LineaBesuHardForkIdProvider
import net.consensys.linea.LineaL1FinalizationUpdaterService
import net.consensys.linea.PluginCliOptions
import net.consensys.linea.async.toSafeFuture
import org.hyperledger.besu.plugin.BesuPlugin
import org.hyperledger.besu.plugin.ServiceManager
import org.hyperledger.besu.plugin.services.BlockchainService
import org.hyperledger.besu.plugin.services.PicoCLIOptions

class LineaL1FinalizationTagUpdaterPlugin : BesuPlugin {
  private val cliOptions = PluginCliOptions()
  private val vertx: Vertx = Vertx.vertx()
  private lateinit var service: LineaL1FinalizationUpdaterService
  private lateinit var blockchainService: BlockchainService

  override fun register(serviceManager: ServiceManager) {
    val cmdlineOptions = serviceManager.getService(PicoCLIOptions::class.java)
      .orElseThrow { IllegalStateException("Failed to obtain PicoCLI options from the BesuContext") }
    cmdlineOptions.addPicoCLIOptions(CLI_OPTIONS_PREFIX, cliOptions)

    blockchainService = serviceManager.getService(BlockchainService::class.java)
      .orElseThrow { RuntimeException("Failed to obtain BlockchainService from the BesuContext.") }
  }

  override fun start() {
    service = LineaL1FinalizationUpdaterService(
      vertx,
      cliOptions.getConfig(),
      LineaBesuHardForkIdProvider(blockchainService),
      LineaBesuEngineBlockTagUpdater(blockchainService),
    )
    service.start()
  }

  override fun stop() {
    service.stop()
      .thenCompose {
        vertx.close().toSafeFuture()
      }.get()
  }

  companion object {
    private const val CLI_OPTIONS_PREFIX = "linea"
  }
}
