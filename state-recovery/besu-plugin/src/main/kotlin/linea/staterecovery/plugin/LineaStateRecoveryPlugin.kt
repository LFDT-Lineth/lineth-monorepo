package linea.staterecovery.plugin

import io.micrometer.core.instrument.simple.SimpleMeterRegistry
import io.vertx.core.Vertx
import linea.staterecovery.BlockHeaderStaticFields
import linea.staterecovery.FileBasedRecoveryStatusPersistence
import linea.staterecovery.RecoveryStatusPersistence
import linea.staterecovery.StateRecoveryApp
import linea.staterecovery.clients.ExecutionLayerInProcessClient
import net.consensys.linea.async.get
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.hyperledger.besu.plugin.BesuPlugin
import org.hyperledger.besu.plugin.ServiceManager
import org.hyperledger.besu.plugin.services.BesuConfiguration
import org.hyperledger.besu.plugin.services.BesuEvents
import org.hyperledger.besu.plugin.services.BesuService
import org.hyperledger.besu.plugin.services.BlockSimulationService
import org.hyperledger.besu.plugin.services.BlockchainService
import org.hyperledger.besu.plugin.services.PicoCLIOptions
import org.hyperledger.besu.plugin.services.mining.MiningService
import org.hyperledger.besu.plugin.services.p2p.P2PService
import org.hyperledger.besu.plugin.services.sync.SynchronizationService

fun <T : BesuService> ServiceManager.getServiceOrThrow(clazz: Class<T>): T {
  return this.getService(clazz)
    .orElseThrow { IllegalStateException("${clazz.name} is not present in BesuContext") }
}

open class LineaStateRecoveryPlugin : BesuPlugin {
  private val log: Logger = LogManager.getLogger(LineaStateRecoveryPlugin::class.java)
  private val vertx = Vertx.vertx()
  private val cliOptions = PluginCliOptions()
  private lateinit var serviceManager: ServiceManager
  private lateinit var recoveryModeManager: RecoveryModeManager
  private lateinit var recoveryStatusPersistence: RecoveryStatusPersistence
  private lateinit var stateRecoverApp: StateRecoveryApp

  override fun register(serviceManager: ServiceManager) {
    log.debug("registering")
    this.serviceManager = serviceManager
    serviceManager
      .getServiceOrThrow(PicoCLIOptions::class.java)
      .addPicoCLIOptions(PluginCliOptions.cliPluginPrefixName, cliOptions)
    log.debug("registered")
  }

  override fun start() {
    val config = cliOptions.getConfig()
    val blockchainService = serviceManager.getServiceOrThrow(BlockchainService::class.java)
    val blockHeaderStaticFields = BlockHeaderStaticFields(
      coinbase = config.lineaSequencerBeneficiaryAddress.toArray(),
      gasLimit = blockchainService.chainHeadHeader.gasLimit.toULong(),
      difficulty = 2UL // Note, this will need to change once we move to QBFT
    )
    this.recoveryStatusPersistence = FileBasedRecoveryStatusPersistence(
      serviceManager.getServiceOrThrow(BesuConfiguration::class.java)
        .dataPath
        .resolve("plugin-staterecovery-status.json")
    )
    log.info(
      "starting: config={} blockHeaderStaticFields={} previousRecoveryStartBlockNumber={}",
      config,
      blockHeaderStaticFields,
      this.recoveryStatusPersistence.getRecoveryStartBlockNumber()
    )

    val synchronizationService = serviceManager.getServiceOrThrow(SynchronizationService::class.java)
    this.recoveryModeManager = RecoveryModeManager(
      p2pService = serviceManager.getServiceOrThrow(P2PService::class.java),
      miningService = serviceManager.getServiceOrThrow(MiningService::class.java),
      recoveryStatePersistence = this.recoveryStatusPersistence,
      synchronizationService = synchronizationService,
      headBlockNumber = blockchainService.chainHeadHeader.number.toULong()
    )
    val simulatorService = serviceManager.getServiceOrThrow(BlockSimulationService::class.java)
    val executionLayerClient = ExecutionLayerInProcessClient.create(
      blockchainService = blockchainService,
      stateRecoveryModeManager = this.recoveryModeManager,
      stateRecoveryStatusPersistence = this.recoveryStatusPersistence,
      simulatorService = simulatorService,
      synchronizationService = synchronizationService
    )

    this.stateRecoverApp = run {
      createAppAllInProcess(
        vertx = vertx,
        // Metrics won't be exposed. Needs proper integration with Besu Metrics, not priority now.
        meterRegistry = SimpleMeterRegistry(),
        elClient = executionLayerClient,
        stateManagerClientEndpoint = config.shomeiEndpoint,
        l1RpcEndpoint = config.l1RpcEndpoint,
        blobScanEndpoint = config.blobscanEndpoint,
        blockHeaderStaticFields = blockHeaderStaticFields,
        appConfig = StateRecoveryApp.Config(
          smartContractAddress = config.l1SmartContractAddress.toString(),
          l1LatestSearchBlock = net.consensys.linea.BlockParameter.Tag.LATEST,
          overridingRecoveryStartBlockNumber = config.overridingRecoveryStartBlockNumber,
          l1PollingInterval = config.l1PollingInterval
        )
      )
    }
    // add recoverty mode manager as listener to block added events
    // so it stops P2P sync when it got target block
    serviceManager
      .getServiceOrThrow(BesuEvents::class.java)
      .addBlockAddedListener(recoveryModeManager)
    this.stateRecoverApp.start().get()
    log.info(
      "started: recoveryStartBlockNumber={}",
      this.recoveryStatusPersistence.getRecoveryStartBlockNumber()
    )
  }

  override fun afterExternalServicePostMainLoop() {
  }

  override fun stop() {
    stateRecoverApp.stop()
      .whenComplete { _, throwable ->
        vertx.close().get()
        if (throwable != null) {
          throw throwable
        }
      }
      .get()
  }
}
