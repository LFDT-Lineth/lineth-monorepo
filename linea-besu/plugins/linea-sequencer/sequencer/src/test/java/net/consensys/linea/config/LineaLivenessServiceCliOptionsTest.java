/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package net.consensys.linea.config;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatExceptionOfType;

import java.math.BigInteger;
import net.consensys.linea.config.LineaLivenessServiceConfiguration.SignerType;
import net.consensys.linea.sequencer.liveness.LineaLivenessTxBuilder;
import org.hyperledger.besu.plugin.services.BlockchainService;
import org.hyperledger.besu.services.PicoCLIOptionsImpl;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import picocli.CommandLine;
import picocli.CommandLine.Command;

class LineaLivenessServiceCliOptionsTest {
  private static final String CONTRACT_ADDRESS = "0x1111111111111111111111111111111111111111";
  private static final String SIGNER_ADDRESS = "0x2222222222222222222222222222222222222222";
  private static final String PUBLIC_KEY =
      "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
          + "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb";

  @Command
  static final class TestCommand {}

  private CommandLine commandLine;
  private LineaLivenessServiceCliOptions options;

  @BeforeEach
  void setUp() {
    commandLine = new CommandLine(new TestCommand());
    options = LineaLivenessServiceCliOptions.create();
    new PicoCLIOptionsImpl(commandLine).addPicoCLIOptions("linea", options);
  }

  @Test
  void web3SignerRemainsTheDefault() {
    commandLine.parseArgs(
        "--plugin-linea-liveness-enabled",
        "--plugin-linea-liveness-contract-address",
        CONTRACT_ADDRESS,
        "--plugin-linea-liveness-signer-address",
        SIGNER_ADDRESS,
        "--plugin-linea-liveness-signer-url",
        "http://localhost:9000",
        "--plugin-linea-liveness-signer-key-id",
        PUBLIC_KEY);

    LineaLivenessServiceConfiguration config = options.toDomainObject();

    assertThat(config.signerType()).isEqualTo(SignerType.WEB3SIGNER);
    assertThat(config.signerName()).isNull();
  }

  @Test
  void builderAndOriginalTransactionBuilderConstructorRemainCompatible() throws Exception {
    LineaLivenessServiceConfiguration config = LineaLivenessServiceConfiguration.builder().build();

    assertThat(config.signerType()).isEqualTo(SignerType.WEB3SIGNER);
    assertThat(
            LineaLivenessTxBuilder.class.getConstructor(
                LineaLivenessServiceConfiguration.class, BlockchainService.class, BigInteger.class))
        .isNotNull();
  }

  @Test
  void acceptsCustomSignerNameWithoutWeb3SignerOptions() {
    commandLine.parseArgs(
        "--plugin-linea-liveness-enabled",
        "--plugin-linea-liveness-contract-address",
        CONTRACT_ADDRESS,
        "--plugin-linea-liveness-signer-address",
        SIGNER_ADDRESS,
        "--plugin-linea-liveness-signer-type",
        "CUSTOM",
        "--plugin-linea-liveness-signer-name",
        "liveness");

    LineaLivenessServiceConfiguration config = options.toDomainObject();

    assertThat(config.signerType()).isEqualTo(SignerType.CUSTOM);
    assertThat(config.signerName()).isEqualTo("liveness");
  }

  @Test
  void rejectsCustomSignerWithoutName() {
    commandLine.parseArgs(
        "--plugin-linea-liveness-enabled",
        "--plugin-linea-liveness-contract-address",
        CONTRACT_ADDRESS,
        "--plugin-linea-liveness-signer-address",
        SIGNER_ADDRESS,
        "--plugin-linea-liveness-signer-type",
        "CUSTOM");

    assertThatExceptionOfType(IllegalArgumentException.class)
        .isThrownBy(options::toDomainObject)
        .withMessageContaining("--plugin-linea-liveness-signer-name");
  }

  @Test
  void rejectsWeb3SignerOptionsForCustomSigner() {
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
        "http://localhost:9000");

    assertThatExceptionOfType(IllegalArgumentException.class)
        .isThrownBy(options::toDomainObject)
        .withMessageContaining("not valid for CUSTOM");
  }

  @Test
  void redactsTlsPasswordsFromStringRepresentation() {
    commandLine.parseArgs(
        "--plugin-linea-liveness-tls-key-store-password",
        "key-store-secret",
        "--plugin-linea-liveness-tls-trust-store-password",
        "trust-store-secret");

    assertThat(options.toString())
        .doesNotContain("key-store-secret", "trust-store-secret")
        .contains("<redacted>");
  }
}
