/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.config

import com.github.michaelbull.result.Err
import com.github.michaelbull.result.Ok
import com.github.michaelbull.result.Result
import com.github.michaelbull.result.getOrElse
import com.github.michaelbull.result.recoverIf
import com.sksamuel.hoplite.ConfigLoader
import com.sksamuel.hoplite.ConfigLoaderBuilder
import com.sksamuel.hoplite.ConfigResult
import com.sksamuel.hoplite.ExperimentalHoplite
import com.sksamuel.hoplite.fp.Validated
import maru.config.consensus.ForkConfigDecoder
import maru.config.decoders.TomlByteArrayHexDecoder
import org.apache.logging.log4j.Level
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import java.nio.file.Path

fun ConfigLoaderBuilder.addTomlDecoders(strict: Boolean): ConfigLoaderBuilder =
  this
    .addDecoder(TomlByteArrayHexDecoder())
    .addDecoder(ForkConfigDecoder)
    .apply { if (strict) this.strict() }

@OptIn(ExperimentalHoplite::class)
inline fun <reified T : Any> loadConfigsOrError(
  configFiles: List<Path>,
  confLoader: ConfigLoader,
): Result<T, String> =
  confLoader
    .loadConfig<T>(configFiles.reversed().map { it.toAbsolutePath().toString() })
    .let { configResult: ConfigResult<T> ->
      when (configResult) {
        is Validated.Valid -> Ok(configResult.value)
        is Validated.Invalid -> Err(configResult.getInvalidUnsafe().description())
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
  logger: Logger = LogManager.getLogger("maru.config"),
  strict: Boolean = false,
  confLoader: ConfigLoader,
): Result<T, String> =
  loadConfigsOrError<T>(configFiles, confLoader)
    .also {
      val logLevel = if (strict) Level.WARN else Level.ERROR
      logErrorIfPresent(it, logger, logLevel)
    }

inline fun <reified T : Any> loadConfigs(
  configFiles: List<Path>,
  logger: Logger = LogManager.getLogger("maru.config"),
  enforceStrict: Boolean = false,
  confLoaderBuilderFn: (Boolean) -> ConfigLoader,
): T =
  loadConfigsAndLogErrors<T>(
    configFiles,
    logger,
    strict = true,
    confLoader = confLoaderBuilderFn(true),
  ).recoverIf({ !enforceStrict }, {
    loadConfigsAndLogErrors<T>(
      configFiles,
      logger,
      strict = false,
      confLoader = confLoaderBuilderFn(false),
    ).getOrElse {
      throw RuntimeException("Invalid configurations: $it")
    }
  })
    .getOrElse {
      throw RuntimeException("Invalid configurations: $it")
    }
