package linea.coordinator.config.v2.toml

import com.github.michaelbull.result.Err
import com.github.michaelbull.result.Ok
import com.github.michaelbull.result.Result
import com.github.michaelbull.result.get
import com.github.michaelbull.result.getOrElse
import com.github.michaelbull.result.recoverIf
import com.sksamuel.hoplite.ConfigLoaderBuilder
import com.sksamuel.hoplite.ConfigResult
import com.sksamuel.hoplite.ExperimentalHoplite
import com.sksamuel.hoplite.fp.Validated
import com.sksamuel.hoplite.toml.TomlPropertySource
import linea.coordinator.config.v2.CoordinatorConfig
import linea.coordinator.config.v2.toml.decoders.BlockParameterDecoder
import linea.coordinator.config.v2.toml.decoders.BlockParameterNumberDecoder
import linea.coordinator.config.v2.toml.decoders.BlockParameterTagDecoder
import linea.coordinator.config.v2.toml.decoders.TomlByteArrayHexDecoder
import linea.coordinator.config.v2.toml.decoders.TomlKotlinDurationDecoder
import linea.coordinator.config.v2.toml.decoders.TomlSignerTypeDecoder
import org.apache.logging.log4j.Level
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import java.nio.file.Path

fun ConfigLoaderBuilder.addCoordinatorTomlDecoders(strict: Boolean): ConfigLoaderBuilder {
  return this
    .addDecoder(BlockParameterTagDecoder())
    .addDecoder(BlockParameterNumberDecoder())
    .addDecoder(BlockParameterDecoder())
    .addDecoder(TomlByteArrayHexDecoder())
    .addDecoder(TomlKotlinDurationDecoder())
    .addDecoder(TomlSignerTypeDecoder())
    .apply { if (strict) this.strict() }
}

@OptIn(ExperimentalHoplite::class)
inline fun <reified T : Any> parseConfig(toml: String, strict: Boolean = true): T {
  return ConfigLoaderBuilder
    .default()
    .withExplicitSealedTypes()
    .addCoordinatorTomlDecoders(strict)
    .addSource(TomlPropertySource(toml))
    .build()
    .loadConfigOrThrow<T>()
}

@OptIn(ExperimentalHoplite::class)
inline fun <reified T : Any> loadConfigsOrError(
  configFiles: List<Path>,
  strict: Boolean,
): Result<T, String> {
  val confLoader = ConfigLoaderBuilder
    .empty()
    .addDefaults()
    .withExplicitSealedTypes()
    .addCoordinatorTomlDecoders(strict)
    .build()

  return confLoader
    .loadConfig<T>(configFiles.reversed().map { it.toAbsolutePath().toString() })
    .let { configResult: ConfigResult<T> ->
      when (configResult) {
        is Validated.Valid -> Ok(configResult.value)
        is Validated.Invalid -> Err(configResult.getInvalidUnsafe().description())
      }
    }
}

fun logErrorIfPresent(
  configLoadingResult: Result<Any?, String>,
  logger: Logger,
  logLevel: Level = Level.ERROR,
) {
  if (configLoadingResult is Err) {
    logger.log(logLevel, configLoadingResult.error)
  }
}

inline fun <reified T : Any> loadConfigsAndLogErrors(
  configFiles: List<Path>,
  logger: Logger = LogManager.getLogger("linea.coordinator.config"),
  strict: Boolean,
): Result<T, String> {
  return loadConfigsOrError<T>(configFiles, strict = strict)
    .also {
      val logLevel = if (strict) Level.WARN else Level.ERROR
      logErrorIfPresent(it, logger, logLevel)
    }
}

fun loadConfigsOrError(
  coordinatorConfigFiles: List<Path>,
  tracesLimitsFileV2: Path,
  gasPriceCapTimeOfDayMultipliersFile: Path,
  smartContractErrorsFile: Path,
  logger: Logger = LogManager.getLogger("linea.coordinator.config"),
  strict: Boolean = false,
): Result<CoordinatorConfigToml, String> {
  val coordinatorBaseConfigs =
    loadConfigsAndLogErrors<CoordinatorConfigFileToml>(coordinatorConfigFiles, logger, strict)
  val tracesLimitsV2Configs =
    loadConfigsAndLogErrors<TracesLimitsConfigFileToml>(listOf(tracesLimitsFileV2), logger, strict)
  val gasPriceCapTimeOfDayMultipliersConfig =
    loadConfigsAndLogErrors<GasPriceCapTimeOfDayMultipliersConfigFileToml>(
      listOf(gasPriceCapTimeOfDayMultipliersFile),
      logger,
      strict,
    )
  val smartContractErrorsConfig = loadConfigsAndLogErrors<SmartContractErrorCodesConfigFileToml>(
    listOf(smartContractErrorsFile),
    logger,
    strict,
  )
  val configError = listOf(
    coordinatorBaseConfigs,
    tracesLimitsV2Configs,
    gasPriceCapTimeOfDayMultipliersConfig,
    smartContractErrorsConfig,
  )
    .find { it is Err }

  if (configError != null) {
    @Suppress("UNCHECKED_CAST")
    return configError as Result<CoordinatorConfigToml, String>
  }

  val finalConfig = CoordinatorConfigToml(
    configs = coordinatorBaseConfigs.get()!!,
    tracesLimitsV2 = tracesLimitsV2Configs.get()!!,
    l1DynamicGasPriceCapTimeOfDayMultipliers = gasPriceCapTimeOfDayMultipliersConfig.get(),
    smartContractErrors = smartContractErrorsConfig.get(),
  )
  return Ok(finalConfig)
}

fun loadConfigs(
  coordinatorConfigFiles: List<Path>,
  tracesLimitsFileV2: Path,
  gasPriceCapTimeOfDayMultipliersFile: Path,
  smartContractErrorsFile: Path,
  logger: Logger = LogManager.getLogger("linea.coordinator.config"),
  enforceStrict: Boolean = false,
): CoordinatorConfig {
  return loadConfigsOrError(
    coordinatorConfigFiles,
    tracesLimitsFileV2,
    gasPriceCapTimeOfDayMultipliersFile,
    smartContractErrorsFile,
    logger,
    strict = true,
  )
    .recoverIf({ !enforceStrict }, {
      loadConfigsOrError(
        coordinatorConfigFiles,
        tracesLimitsFileV2,
        gasPriceCapTimeOfDayMultipliersFile,
        smartContractErrorsFile,
        logger,
        strict = false,
      ).getOrElse {
        throw RuntimeException("Invalid configurations: $it")
      }
    })
    .getOrElse {
      throw RuntimeException("Invalid configurations: $it")
    }
    .reified()
}
