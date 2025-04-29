package linea.coordinator.config

import com.github.michaelbull.result.Err
import com.github.michaelbull.result.Ok
import com.github.michaelbull.result.Result
import com.github.michaelbull.result.get
import com.github.michaelbull.result.getOrElse
import com.sksamuel.hoplite.ConfigLoaderBuilder
import com.sksamuel.hoplite.addPathSource
import net.consensys.linea.traces.TracesCountersV1
import net.consensys.linea.traces.TracesCountersV2
import net.consensys.zkevm.coordinator.app.config.BlockParameterDecoder
import net.consensys.zkevm.coordinator.app.config.CoordinatorConfig
import net.consensys.zkevm.coordinator.app.config.CoordinatorConfigTomlDto
import net.consensys.zkevm.coordinator.app.config.GasPriceCapTimeOfDayMultipliersConfig
import net.consensys.zkevm.coordinator.app.config.SmartContractErrorCodesConfig
import net.consensys.zkevm.coordinator.app.config.TracesLimitsV1ConfigFile
import net.consensys.zkevm.coordinator.app.config.TracesLimitsV2ConfigFile
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import java.nio.file.Path

inline fun <reified T : Any> loadConfigsOrError(
  configFiles: List<Path>
): Result<T, String> {
  val confBuilder: ConfigLoaderBuilder = ConfigLoaderBuilder.Companion
    .empty()
    .addDefaults()
    .addDecoder(BlockParameterDecoder())
  for (configFile in configFiles.reversed()) {
    // files must be added in reverse order for overriding
    confBuilder.addPathSource(configFile, false)
  }

  return confBuilder.build().loadConfig<T>(emptyList()).let { config ->
    if (config.isInvalid()) {
      Err(config.getInvalidUnsafe().description())
    } else {
      Ok(config.getUnsafe())
    }
  }
}

fun logErrorIfPresent(
  configName: String,
  configFiles: List<Path>,
  configLoadingResult: Result<Any?, String>,
  logger: Logger
) {
  if (configLoadingResult is Err) {
    logger.error("Failed to load $configName from files=$configFiles with error=${configLoadingResult.error}")
  }
}

inline fun <reified T : Any> loadConfigsAndLogErrors(
  configFiles: List<Path>,
  configName: String,
  logger: Logger = LogManager.getLogger("linea.coordinator.config")
): Result<T, String> {
  return loadConfigsOrError<T>(configFiles)
    .also { logErrorIfPresent(configName, configFiles, it, logger) }
}

fun loadConfigsOrError(
  coordinatorConfigFiles: List<Path>,
  tracesLimitsFileV1: Path?,
  tracesLimitsFileV2: Path?,
  gasPriceCapTimeOfDayMultipliersFile: Path,
  smartContractErrorsFile: Path,
  logger: Logger = LogManager.getLogger("linea.coordinator.config")
): Result<CoordinatorConfigTomlDto, String> {
  val coordinatorBaseConfigs =
    loadConfigsAndLogErrors<CoordinatorConfigTomlDto>(coordinatorConfigFiles, "coordinator", logger)
  val tracesLimitsV1Configs = tracesLimitsFileV1
    ?.let { loadConfigsAndLogErrors<TracesLimitsV1ConfigFile>(listOf(it), "traces limit v1", logger) }
  val tracesLimitsV2Configs = tracesLimitsFileV2
    ?.let { loadConfigsAndLogErrors<TracesLimitsV2ConfigFile>(listOf(it), "traces limits v2", logger) }
  val gasPriceCapTimeOfDayMultipliersConfig =
    loadConfigsAndLogErrors<GasPriceCapTimeOfDayMultipliersConfig>(
      listOf(gasPriceCapTimeOfDayMultipliersFile),
      "l1 submission gas prices caps",
      logger
    )
  val smartContractErrorsConfig = loadConfigsAndLogErrors<SmartContractErrorCodesConfig>(
    listOf(smartContractErrorsFile),
    "smart contract errors",
    logger
  )
  val configError = listOf(
    coordinatorBaseConfigs,
    tracesLimitsV1Configs,
    tracesLimitsV1Configs,
    gasPriceCapTimeOfDayMultipliersConfig,
    smartContractErrorsConfig
  )
    .find { it is Err }

  if (configError != null) {
    @Suppress("UNCHECKED_CAST")
    return configError as Result<CoordinatorConfigTomlDto, String>
  }

  val baseConfig = coordinatorBaseConfigs.get()!!
  val finalConfig = baseConfig.copy(
    conflation = baseConfig.conflation.copy(
      _tracesLimitsV1 = tracesLimitsV1Configs?.get()?.tracesLimits?.let { TracesCountersV1(it) },
      _tracesLimitsV2 = tracesLimitsV2Configs?.get()?.tracesLimits?.let { TracesCountersV2(it) },
      _smartContractErrors = smartContractErrorsConfig.get()!!.smartContractErrors
    ),
    l1DynamicGasPriceCapService = baseConfig.l1DynamicGasPriceCapService.copy(
      gasPriceCapCalculation = baseConfig.l1DynamicGasPriceCapService.gasPriceCapCalculation.copy(
        timeOfDayMultipliers = gasPriceCapTimeOfDayMultipliersConfig.get()?.gasPriceCapTimeOfDayMultipliers
      )
    )
  )
  return Ok(finalConfig)
}

fun loadConfigs(
  coordinatorConfigFiles: List<Path>,
  tracesLimitsFileV1: Path?,
  tracesLimitsFileV2: Path?,
  gasPriceCapTimeOfDayMultipliersFile: Path,
  smartContractErrorsFile: Path,
  logger: Logger = LogManager.getLogger("linea.coordinator.config")
): CoordinatorConfig {
  loadConfigsOrError(
    coordinatorConfigFiles,
    tracesLimitsFileV1,
    tracesLimitsFileV2,
    gasPriceCapTimeOfDayMultipliersFile,
    smartContractErrorsFile,
    logger
  ).let {
    return it
      .getOrElse {
        throw RuntimeException("Invalid configurations: $it")
      }.reified()
  }
}
