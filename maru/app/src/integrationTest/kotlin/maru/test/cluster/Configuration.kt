/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.test.cluster

import org.apache.logging.log4j.Level
import org.apache.logging.log4j.core.config.Configurator
import org.apache.logging.log4j.core.config.builder.api.ConfigurationBuilderFactory
import org.apache.logging.log4j.core.config.builder.impl.BuiltConfiguration

fun configureLoggers(
  pattern: String = "%d{yyyy-MM-dd HH:mm:ss} [%level] - %msg | %logger{36}%n",
  rootLevel: Level = Level.INFO,
  vararg logLevels: Pair<String, Level>,
) = configureLoggers(
  pattern,
  rootLevel,
  logLevels.toList(),
)

fun configureLoggers(
  pattern: String = "%d{yyyy-MM-dd HH:mm:ss} [%level] - %msg | %logger{36}%n",
  rootLevel: Level = Level.INFO,
  logLevels: List<Pair<String, Level>> = emptyList(),
) {
  val builder = ConfigurationBuilderFactory.newConfigurationBuilder()

  // Create console appender with custom pattern
  val appenderBuilder =
    builder
      .newAppender("Console", "CONSOLE")
      .addAttribute("target", "SYSTEM_OUT")

  val layoutBuilder =
    builder
      .newLayout("PatternLayout")
      .addAttribute("pattern", pattern)
  appenderBuilder.add(layoutBuilder)

  builder.add(appenderBuilder)

  // Configure root logger
  val rootLoggerBuilder =
    builder
      .newRootLogger(rootLevel)
      .add(builder.newAppenderRef("Console"))
  builder.add(rootLoggerBuilder)

  // Configure specific loggers
  logLevels.forEach { (loggerName, level) ->
    val loggerBuilder =
      builder
        .newLogger(loggerName, level)
        .addAttribute("additivity", false)
        .add(builder.newAppenderRef("Console"))
    builder.add(loggerBuilder)
  }

  val configuration: BuiltConfiguration = builder.build()
  Configurator.initialize(configuration)
}
