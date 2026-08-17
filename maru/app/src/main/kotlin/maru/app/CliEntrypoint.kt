/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.app

import org.apache.logging.log4j.LogManager
import picocli.CommandLine
import kotlin.system.exitProcess

object CliEntrypoint {
  private val log = LogManager.getLogger(this.javaClass)

  @JvmStatic
  fun main(args: Array<String>) {
    val exitCode = execute(args)
    if (exitCode != 0) {
      exitProcess(exitCode)
    }
  }

  fun execute(
    args: Array<String>,
    maruAppFactory: MaruAppFactoryCreator = MaruAppFactory(),
  ): Int {
    val cmd = CommandLine(MaruAppCli(maruAppFactory))
    cmd.registerConverter(
      Network::class.java,
      KebabToEnumConverter(Network::class.java),
    )
    cmd.setExecutionExceptionHandler { ex, _, _ ->
      log.error("Execution failure: ", ex)
      1
    }
    cmd.setParameterExceptionHandler { ex, _ ->
      log.error("Invalid args!: ", ex)
      1
    }
    return cmd.execute(*args)
  }
}
