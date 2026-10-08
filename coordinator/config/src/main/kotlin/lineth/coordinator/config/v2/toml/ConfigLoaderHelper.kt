package lineth.coordinator.config.v2.toml

import com.github.michaelbull.result.Err
import com.github.michaelbull.result.Ok
import com.github.michaelbull.result.Result
import com.github.michaelbull.result.get
import com.github.michaelbull.result.getOrElse
import com.github.michaelbull.result.map
import com.github.michaelbull.result.recoverIf
import com.sksamuel.hoplite.ConfigLoader
import com.sksamuel.hoplite.ConfigLoaderBuilder
import com.sksamuel.hoplite.ConfigResult
import com.sksamuel.hoplite.ExperimentalHoplite
import com.sksamuel.hoplite.PropertySource
import com.sksamuel.hoplite.fp.Validated
import com.sksamuel.hoplite.toml.TomlPropertySource
import linea.hoplite.toml.TomlByteArrayHexDecoder
import linea.hoplite.toml.TomlKotlinDurationDecoder
import linea.hoplite.toml.TomlKotlinInstantDecoder
import lineth.coordinator.config.v2.CoordinatorConfig
import lineth.coordinator.config.v2.toml.decoders.BlockParameterDecoder
import lineth.coordinator.config.v2.toml.decoders.BlockParameterNumberDecoder
import lineth.coordinator.config.v2.toml.decoders.BlockParameterTagDecoder
import lineth.coordinator.config.v2.toml.decoders.TomlProverTypeDecoder
import lineth.coordinator.config.v2.toml.decoders.TomlSignerTypeDecoder
import org.apache.logging.log4j.Level
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import java.nio.file.Path

@OptIn(ExperimentalHoplite::class)
fun configLoaderBuilder(
  strict: Boolean,
  addDefaultPreprocessors: Boolean,
  addDefaultPropertySources: Boolean,
): ConfigLoaderBuilder {
  return ConfigLoaderBuilder
    .empty()
    .addDefaultDecoders()
    .addDefaultNodeTransformers()
    .addDefaultParamMappers()
    .addDefaultParsers()
    .apply { if (addDefaultPropertySources) addDefaultPropertySources() }
    .apply {
      if (addDefaultPreprocessors) addDefaultPreprocessors()
    }
    .withExplicitSealedTypes()
    .addDecoder(BlockParameterTagDecoder())
    .addDecoder(BlockParameterNumberDecoder())
    .addDecoder(BlockParameterDecoder())
    .addDecoder(TomlByteArrayHexDecoder())
    .addDecoder(TomlKotlinDurationDecoder())
    .addDecoder(TomlKotlinInstantDecoder())
    .addDecoder(TomlSignerTypeDecoder())
    .addDecoder(TomlProverTypeDecoder())
    .apply { if (strict) this.strict() }
}

inline fun <reified T : Any> parseConfig(
  toml: String,
  strict: Boolean = true,
  addDefaultPreprocessors: Boolean = false,
  addDefaultPropertySources: Boolean = false,
): T {
  return configLoaderBuilder(strict, addDefaultPreprocessors, addDefaultPropertySources)
    .addSource(TomlPropertySource(toml))
    .build()
    .loadConfigOrThrow<T>()
}

@OptIn(ExperimentalHoplite::class)
@PublishedApi
internal fun buildConfigLoader(
  configFiles: List<Path>,
  strict: Boolean,
  ignoredTopLevelKeys: Set<String>,
  onlyTopLevelKeys: Set<String>?,
  deprecatedAliases: List<DeprecatedKeyAlias>,
  logger: Logger?,
  addDefaultPreprocessors: Boolean,
  addDefaultPropertySources: Boolean,
): ConfigLoader {
  // Hoplite gives priority to the first source, so the last config file is added first
  val fileSources =
    configFiles.reversed().map { file ->
      PropertySource.path(file.toAbsolutePath())
        .withDeprecatedAliases(deprecatedAliases, logger)
        .withoutTopLevelKeys(ignoredTopLevelKeys)
        .let { source -> if (onlyTopLevelKeys != null) source.onlyTopLevelKeys(onlyTopLevelKeys) else source }
    }
  return configLoaderBuilder(strict, addDefaultPreprocessors, addDefaultPropertySources)
    .addPropertySources(fileSources)
    .build()
}

/**
 * @param ignoredTopLevelKeys top-level tables dropped before decoding (e.g. owned by a [ConfigExtension]),
 *   so they do not trigger the unknown key handling
 * @param onlyTopLevelKeys when set, only these top-level tables are kept (used to load [ConfigExtension] sections)
 * @param deprecatedAliases renamed keys still accepted from the config files
 * @param logger used for deprecation warnings; pass it on the strict pass only, as the lenient pass re-reads the files
 */
inline fun <reified T : Any> loadConfigsOrError(
  configFiles: List<Path>,
  strict: Boolean,
  addDefaultPreprocessors: Boolean,
  addDefaultPropertySources: Boolean,
  ignoredTopLevelKeys: Set<String> = emptySet(),
  onlyTopLevelKeys: Set<String>? = null,
  deprecatedAliases: List<DeprecatedKeyAlias> = emptyList(),
  logger: Logger? = null,
): Result<T, String> {
  return buildConfigLoader(
    configFiles,
    strict,
    ignoredTopLevelKeys,
    onlyTopLevelKeys,
    deprecatedAliases,
    logger,
    addDefaultPreprocessors,
    addDefaultPropertySources,
  )
    .loadConfig<T>()
    .let { configResult: ConfigResult<T> ->
      when (configResult) {
        is Validated.Valid -> Ok(configResult.value)
        is Validated.Invalid -> Err(configResult.getInvalidUnsafe().description())
      }
    }
}

fun logErrorIfPresent(configLoadingResult: Result<Any?, String>, logger: Logger, logLevel: Level = Level.ERROR) {
  if (configLoadingResult is Err) {
    logger.log(logLevel, configLoadingResult.error)
  }
}

inline fun <reified T : Any> loadConfigsAndLogErrors(
  configFiles: List<Path>,
  logger: Logger = LogManager.getLogger("lineth.coordinator.config"),
  strict: Boolean,
  addDefaultPreprocessors: Boolean,
  addDefaultPropertySources: Boolean,
  ignoredTopLevelKeys: Set<String> = emptySet(),
  onlyTopLevelKeys: Set<String>? = null,
  deprecatedAliases: List<DeprecatedKeyAlias> = emptyList(),
): Result<T, String> {
  return loadConfigsOrError<T>(
    configFiles,
    strict = strict,
    addDefaultPreprocessors = addDefaultPreprocessors,
    addDefaultPropertySources = addDefaultPropertySources,
    ignoredTopLevelKeys = ignoredTopLevelKeys,
    onlyTopLevelKeys = onlyTopLevelKeys,
    deprecatedAliases = deprecatedAliases,
    // the lenient pass re-reads the same files, warn once
    logger = if (strict) logger else null,
  )
    .also {
      val logLevel = if (strict) Level.WARN else Level.ERROR
      logErrorIfPresent(it, logger, logLevel)
    }
}

const val BUNDLED_SMART_CONTRACT_ERRORS_RESOURCE = "/smart-contract-errors.toml"

fun loadBundledSmartContractErrors(): SmartContractErrorCodesConfigFileToml {
  val toml =
    requireNotNull(
      SmartContractErrorCodesConfigFileToml::class.java.getResourceAsStream(BUNDLED_SMART_CONTRACT_ERRORS_RESOURCE),
    ) {
      "Bundled smart-contract-errors resource $BUNDLED_SMART_CONTRACT_ERRORS_RESOURCE not found on classpath"
    }
      .use { it.readBytes().toString(Charsets.UTF_8) }
  return parseConfig<SmartContractErrorCodesConfigFileToml>(toml, strict = false)
}

/**
 * Loads the smart contract errors bundled with the coordinator, merged with the optional override file.
 * Entries from the override file win over the bundled ones.
 */
fun loadSmartContractErrors(
  overrideFile: Path?,
  logger: Logger = LogManager.getLogger("lineth.coordinator.config"),
  strict: Boolean,
): Result<SmartContractErrorCodesConfigFileToml, String> {
  val bundled = loadBundledSmartContractErrors().smartContractErrors
  if (overrideFile == null) {
    logger.debug("Smart contract errors: {} entries from bundled mapping, no override file", bundled.size)
    return Ok(SmartContractErrorCodesConfigFileToml(bundled))
  }
  return loadConfigsAndLogErrors<SmartContractErrorCodesConfigFileToml>(
    listOf(overrideFile),
    logger,
    strict,
    addDefaultPreprocessors = false,
    addDefaultPropertySources = false,
  )
    .map { configOverrides ->
      logger.debug(
        "Smart contract errors: {} entries from bundled mapping, {} entries from override file {}",
        bundled.size,
        configOverrides.smartContractErrors.size,
        overrideFile,
      )
      SmartContractErrorCodesConfigFileToml(bundled + configOverrides.smartContractErrors)
    }
}

fun loadConfigsOrError(
  coordinatorConfigFiles: List<Path>,
  tracesLimitsFileV4: Path?,
  tracesLimitsFileV5: Path?,
  gasPriceCapTimeOfDayMultipliersFile: Path,
  smartContractErrorsFile: Path? = null,
  logger: Logger = LogManager.getLogger("lineth.coordinator.config"),
  strict: Boolean = false,
  ignoredTopLevelKeys: Set<String> = emptySet(),
): Result<CoordinatorConfigToml, String> {
  val coordinatorBaseConfigs =
    loadConfigsAndLogErrors<CoordinatorConfigFilesToml>(
      coordinatorConfigFiles,
      logger,
      strict,
      ignoredTopLevelKeys = ignoredTopLevelKeys,
      deprecatedAliases = coordinatorDeprecatedKeyAliases,
      addDefaultPreprocessors = true,
      addDefaultPropertySources = true,
    )
  val tracesLimitsV4Configs =
    tracesLimitsFileV4?.let {
      loadConfigsAndLogErrors<TracesLimitsConfigFileV4Toml>(
        listOf(it),
        logger,
        strict,
        addDefaultPreprocessors = false,
        addDefaultPropertySources = false,
      )
    }
  val tracesLimitsV5Configs =
    tracesLimitsFileV5?.let {
      loadConfigsAndLogErrors<TracesLimitsConfigFileV5Toml>(
        listOf(it),
        logger,
        strict,
        addDefaultPreprocessors = false,
        addDefaultPropertySources = false,
      )
    }
  val gasPriceCapTimeOfDayMultipliersConfig =
    loadConfigsAndLogErrors<GasPriceCapTimeOfDayMultipliersConfigFileToml>(
      listOf(gasPriceCapTimeOfDayMultipliersFile),
      logger,
      strict,
      addDefaultPreprocessors = false,
      addDefaultPropertySources = false,
    )
  val smartContractErrorsConfig =
    loadSmartContractErrors(smartContractErrorsFile, logger, strict)
  val configError =
    listOf(
      coordinatorBaseConfigs,
      tracesLimitsV4Configs,
      tracesLimitsV5Configs,
      gasPriceCapTimeOfDayMultipliersConfig,
      smartContractErrorsConfig,
    )
      .find { it is Err }

  if (configError != null) {
    @Suppress("UNCHECKED_CAST")
    return configError as Result<CoordinatorConfigToml, String>
  }

  val finalConfig =
    CoordinatorConfigToml(
      configs = requireNotNull(coordinatorBaseConfigs.get()) { "coordinatorBaseConfigs have errors" },
      tracesLimitsV4 = tracesLimitsV4Configs?.get(),
      tracesLimitsV5 = tracesLimitsV5Configs?.get(),
      l1DynamicGasPriceCapTimeOfDayMultipliers = gasPriceCapTimeOfDayMultipliersConfig.get(),
      smartContractErrors = smartContractErrorsConfig.get(),
    )
  return Ok(finalConfig)
}

fun loadConfigs(
  coordinatorConfigFiles: List<Path>,
  tracesLimitsFileV4: Path?,
  tracesLimitsFileV5: Path?,
  gasPriceCapTimeOfDayMultipliersFile: Path,
  smartContractErrorsFile: Path? = null,
  logger: Logger = LogManager.getLogger("lineth.coordinator.config"),
  enforceStrict: Boolean = false,
  ignoredTopLevelKeys: Set<String> = emptySet(),
): CoordinatorConfig {
  requireNoCoordinatorKeyCollision(ignoredTopLevelKeys)
  return loadConfigsOrError(
    coordinatorConfigFiles = coordinatorConfigFiles,
    tracesLimitsFileV4 = tracesLimitsFileV4,
    tracesLimitsFileV5 = tracesLimitsFileV5,
    gasPriceCapTimeOfDayMultipliersFile = gasPriceCapTimeOfDayMultipliersFile,
    smartContractErrorsFile = smartContractErrorsFile,
    logger = logger,
    strict = true,
    ignoredTopLevelKeys = ignoredTopLevelKeys,
  )
    .recoverIf({ !enforceStrict }, {
      loadConfigsOrError(
        coordinatorConfigFiles = coordinatorConfigFiles,
        tracesLimitsFileV4 = tracesLimitsFileV4,
        tracesLimitsFileV5 = tracesLimitsFileV5,
        gasPriceCapTimeOfDayMultipliersFile = gasPriceCapTimeOfDayMultipliersFile,
        smartContractErrorsFile = smartContractErrorsFile,
        logger = logger,
        strict = false,
        ignoredTopLevelKeys = ignoredTopLevelKeys,
      ).getOrElse {
        throw RuntimeException("Invalid configurations: $it")
      }
    })
    .getOrElse {
      throw RuntimeException("Invalid configurations: $it")
    }
    .reified()
}
