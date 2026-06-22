/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package linea.maru

import linea.kotlin.decodeHex
import maru.crypto.PrivateKeyGenerator
import maru.crypto.PrivateKeyGenerator.getKeyData
import maru.crypto.PrivateKeyGenerator.getKeyDataByPrefixedKey
import maru.crypto.PrivateKeyGenerator.logKeyData
import picocli.CommandLine
import kotlin.system.exitProcess

@CommandLine.Command(
  name = "keytool",
  description = ["Keytool subcommands"],
  mixinStandardHelpOptions = true,
  version = ["1.0.0"],
)
class KeyTool {
  @CommandLine.Command(
    description = ["Generates secp256k1 private keys and node info"],
    mixinStandardHelpOptions = true,
    version = ["1.0.0"],
  )
  fun generateKeys(
    @CommandLine.Option(names = ["-n", "--numberOfKeys"], description = ["Number of keys to generate"])
    numberOfKeys: Int = 1,
  ): Int {
    repeat(numberOfKeys) {
      PrivateKeyGenerator
        .generatePrivateKey()
        .also { logKeyData(it) }
    }
    return CommandLine.ExitCode.OK
  }

  @CommandLine.Command(
    description = ["Logs secp256k1 private key info and node info"],
    mixinStandardHelpOptions = true,
    version = ["1.0.0"],
  )
  fun secp256k1Info(
    @CommandLine.Option(
      names = ["-k", "--privKey"],
      description = ["secp256k1 private key hex string"],
      required = true,
    )
    privKey: String,
  ): Int {
    val privKeyBytes =
      runCatching { privKey.decodeHex() }
        .getOrElse {
          println("failed to decode private key: $it")
          return CommandLine.ExitCode.USAGE
        }
    logKeyData(getKeyData(privKeyBytes))

    return CommandLine.ExitCode.OK
  }

  @CommandLine.Command(
    description = ["Logs protobuf serialized prefixed secp256k1 private key info and node info"],
    mixinStandardHelpOptions = true,
    version = ["1.0.0"],
  )
  fun prefixedKeyInfo(
    @CommandLine.Option(
      names = ["-k", "--privKey"],
      description = ["prefixed secp256k1 private key hex string"],
      required = true,
    )
    privKey: String,
  ): Int {
    val privKeyBytes =
      runCatching { privKey.decodeHex() }
        .getOrElse {
          println("failed to decode private key: $it")
          return CommandLine.ExitCode.USAGE
        }

    logKeyData(getKeyDataByPrefixedKey(privKeyBytes))

    return CommandLine.ExitCode.OK
  }

  companion object {
    @JvmStatic
    fun main(args: Array<String>) {
      exitProcess(CommandLine(KeyTool()).execute(*args))
    }
  }
}
