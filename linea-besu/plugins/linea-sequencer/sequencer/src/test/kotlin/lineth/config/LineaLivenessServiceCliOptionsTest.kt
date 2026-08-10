/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package lineth.config

import lineth.config.LineaLivenessServiceConfiguration.SignerType
import lineth.sequencer.liveness.LineaLivenessTxBuilder
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatExceptionOfType
import org.hyperledger.besu.plugin.services.BlockchainService
import org.hyperledger.besu.services.PicoCLIOptionsImpl
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import picocli.CommandLine
import picocli.CommandLine.Command
import java.math.BigInteger

class LineaLivenessServiceCliOptionsTest {
  @Command
  private class TestCommand

  private lateinit var commandLine: CommandLine
  private lateinit var options: LineaLivenessServiceCliOptions

  @BeforeEach
  fun setUp() {
    commandLine = CommandLine(TestCommand())
    options = LineaLivenessServiceCliOptions.create()
    PicoCLIOptionsImpl(commandLine).addPicoCLIOptions("linea", options)
  }

  @Test
  fun `Web3Signer remains the default`() {
    commandLine.parseArgs(
      "--plugin-linea-liveness-enabled",
      "--plugin-linea-liveness-contract-address",
      CONTRACT_ADDRESS,
      "--plugin-linea-liveness-signer-address",
      SIGNER_ADDRESS,
      "--plugin-linea-liveness-signer-url",
      "http://localhost:9000",
      "--plugin-linea-liveness-signer-key-id",
      PUBLIC_KEY,
    )

    val config = options.toDomainObject()

    assertThat(config.signerType()).isEqualTo(SignerType.WEB3SIGNER)
    assertThat(config.signerName()).isNull()
  }

  @Test
  fun `builder and original transaction builder constructor remain compatible`() {
    val config = LineaLivenessServiceConfiguration.builder().build()

    assertThat(config.signerType()).isEqualTo(SignerType.WEB3SIGNER)
    assertThat(
      LineaLivenessTxBuilder::class.java.getConstructor(
        LineaLivenessServiceConfiguration::class.java,
        BlockchainService::class.java,
        BigInteger::class.java,
      ),
    )
      .isNotNull()
  }

  @Test
  fun `accepts custom signer name without Web3Signer options`() {
    commandLine.parseArgs(
      "--plugin-linea-liveness-enabled",
      "--plugin-linea-liveness-contract-address",
      CONTRACT_ADDRESS,
      "--plugin-linea-liveness-signer-address",
      SIGNER_ADDRESS,
      "--plugin-linea-liveness-signer-type",
      "CUSTOM",
      "--plugin-linea-liveness-signer-name",
      "liveness",
    )

    val config = options.toDomainObject()

    assertThat(config.signerType()).isEqualTo(SignerType.CUSTOM)
    assertThat(config.signerName()).isEqualTo("liveness")
  }

  @Test
  fun `rejects custom signer without name`() {
    commandLine.parseArgs(
      "--plugin-linea-liveness-enabled",
      "--plugin-linea-liveness-contract-address",
      CONTRACT_ADDRESS,
      "--plugin-linea-liveness-signer-address",
      SIGNER_ADDRESS,
      "--plugin-linea-liveness-signer-type",
      "CUSTOM",
    )

    assertThatExceptionOfType(IllegalArgumentException::class.java)
      .isThrownBy { options.toDomainObject() }
      .withMessageContaining("--plugin-linea-liveness-signer-name")
  }

  @Test
  fun `rejects Web3Signer options for custom signer`() {
    commandLine.parseArgs(
      "--plugin-linea-liveness-enabled",
      "--plugin-linea-liveness-contract-address",
      CONTRACT_ADDRESS,
      "--plugin-linea-liveness-signer-address",
      SIGNER_ADDRESS,
      "--plugin-linea-liveness-signer-type",
      "CUSTOM",
      "--plugin-linea-liveness-signer-name",
      "liveness",
      "--plugin-linea-liveness-signer-url",
      "http://localhost:9000",
    )

    assertThatExceptionOfType(IllegalArgumentException::class.java)
      .isThrownBy { options.toDomainObject() }
      .withMessageContaining("not valid for CUSTOM")
  }

  @Test
  fun `redacts TLS passwords from string representation`() {
    commandLine.parseArgs(
      "--plugin-linea-liveness-tls-key-store-password",
      "key-store-secret",
      "--plugin-linea-liveness-tls-trust-store-password",
      "trust-store-secret",
    )

    assertThat(options.toString())
      .doesNotContain("key-store-secret", "trust-store-secret")
      .contains("<redacted>")
  }

  private companion object {
    const val CONTRACT_ADDRESS = "0x1111111111111111111111111111111111111111"
    const val SIGNER_ADDRESS = "0x2222222222222222222222222222222222222222"
    const val PUBLIC_KEY =
      "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" +
        "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
  }
}
