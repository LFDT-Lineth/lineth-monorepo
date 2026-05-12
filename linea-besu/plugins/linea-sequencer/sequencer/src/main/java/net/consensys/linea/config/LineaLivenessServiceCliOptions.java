/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package net.consensys.linea.config;

import com.google.common.base.MoreObjects;
import jakarta.validation.constraints.*;
import java.nio.file.Path;
import java.time.Duration;
import lombok.Getter;
import net.consensys.linea.plugins.LineaCliOptions;
import picocli.CommandLine.Option;

@Getter
public class LineaLivenessServiceCliOptions implements LineaCliOptions {
  public static final String CONFIG_KEY = "liveness-config";

  public static final String ENABLED = "--plugin-linea-liveness-enabled";
  public static final boolean DEFAULT_ENABLED = false;

  public static final String MAX_BLOCK_AGE_SECONDS =
      "--plugin-linea-liveness-max-block-age-seconds";
  private static final String BUNDLE_MAX_TIMESTAMP_SURPLUS_SECONDS =
      "--plugin-linea-liveness-bundle-max-timestamp-surplus-seconds";
  public static final long DEFAULT_MAX_BLOCK_AGE_SECONDS = 60;
  public static final long DEFAULT_BUNDLE_MAX_TIMESTAMP_SURPLUS_SECONDS = 12;

  public static final String CONTRACT_ADDRESS = "--plugin-linea-liveness-contract-address";

  public static final String SIGNER_URL = "--plugin-linea-liveness-signer-url";
  public static final String SIGNER_KEY_ID = "--plugin-linea-liveness-signer-key-id";
  public static final String SIGNER_ADDRESS = "--plugin-linea-liveness-signer-address";

  public static final String TLS_ENABLED = "--plugin-linea-liveness-tls-enabled";
  public static final boolean DEFAULT_TLS_ENABLED = false;
  public static final String TLS_KEY_STORE_PATH = "--plugin-linea-liveness-tls-key-store-path";
  public static final String TLS_KEY_STORE_PASSWORD =
      "--plugin-linea-liveness-tls-key-store-password";
  public static final String TLS_TRUST_STORE_PATH = "--plugin-linea-liveness-tls-trust-store-path";
  public static final String TLS_TRUST_STORE_PASSWORD =
      "--plugin-linea-liveness-tls-trust-store-password";

  public static final String GAS_LIMIT = "--plugin-linea-liveness-gas-limit";
  public static final long DEFAULT_GAS_LIMIT = 100_000;

  public static final String GAS_PRICE = "--plugin-linea-liveness-gas-price";
  public static final long DEFAULT_GAS_PRICE = 7; // base fee per gas for L2

  @Option(
      names = {ENABLED},
      hidden = true,
      paramLabel = "<BOOLEAN>",
      description = "Enable the Linea liveness service (default: ${DEFAULT-VALUE})",
      arity = "0..1")
  private boolean enabled = DEFAULT_ENABLED;

  @Positive
  @Option(
      names = {MAX_BLOCK_AGE_SECONDS},
      hidden = true,
      paramLabel = "<LONG>",
      description =
          "Maximum age of the last block in seconds before reporting (default: ${DEFAULT-VALUE})",
      defaultValue = "" + DEFAULT_MAX_BLOCK_AGE_SECONDS)
  private long maxBlockAgeSeconds = DEFAULT_MAX_BLOCK_AGE_SECONDS;

  @Positive
  @Option(
      names = {BUNDLE_MAX_TIMESTAMP_SURPLUS_SECONDS},
      hidden = true,
      paramLabel = "<LONG>",
      description =
          "Additional seconds for the max timestamp of bundle (default: ${DEFAULT-VALUE})",
      defaultValue = "" + DEFAULT_BUNDLE_MAX_TIMESTAMP_SURPLUS_SECONDS)
  private long bundleMaxTimestampSurplusSecond = DEFAULT_BUNDLE_MAX_TIMESTAMP_SURPLUS_SECONDS;

  @NotBlank(message = "Contract address must not be blank")
  @Pattern(
      regexp = "^0x[a-fA-F0-9]{40}$",
      message = "Contract address must be a valid Ethereum address")
  @Option(
      names = {CONTRACT_ADDRESS},
      hidden = true,
      paramLabel = "<SMC_ADDRESS>",
      description = "Address of the LineaSequencerUptimeFeed contract (default: ${DEFAULT-VALUE})",
      arity = "1")
  private String contractAddress = null;

  @NotBlank(message = "Web3Signer URL must not be blank")
  @Pattern(regexp = "^https?://.*", message = "Web3Signer URL must be a valid HTTP or HTTPS URL")
  @Option(
      names = {SIGNER_URL},
      hidden = true,
      paramLabel = "<URL>",
      description = "URL of the Web3Signer service, in charge of signing transactions",
      arity = "1")
  private String signerUrl = null;

  @NotBlank(message = "Web3Signer key ID must not be blank")
  @Pattern(
      regexp = "^[a-zA-Z0-9._-]+$",
      message =
          "Web3Signer key ID must contain only alphanumeric characters, dots, underscores, and hyphens")
  @Option(
      names = {SIGNER_KEY_ID},
      hidden = true,
      paramLabel = "<PUBLIC_KEY_HEX>",
      description =
          "Key ID to use with Web3Signer, the public key in hex corresponding to the private key in charge of signing transactions",
      arity = "1")
  private String signerKeyId = null;

  @NotBlank(message = "Web3Signer address must not be blank")
  @Pattern(
      regexp = "^0x[a-fA-F0-9]{40}$",
      message = "Web3Signer address must be a valid Ethereum address")
  @Option(
      names = {SIGNER_ADDRESS},
      hidden = true,
      paramLabel = "<EOA_ADDRESS>",
      description = "Ethereum address corresponding to the Web3Signer key ID",
      arity = "1")
  private String signerAddress = null;

  @Option(
      names = {TLS_ENABLED},
      hidden = true,
      paramLabel = "<BOOLEAN>",
      description = "Enable TLS connection to Web3Signer (default: ${DEFAULT-VALUE})",
      arity = "0..1")
  private boolean tlsEnabled = DEFAULT_TLS_ENABLED;

  @Option(
      names = {TLS_KEY_STORE_PATH},
      hidden = true,
      paramLabel = "<FILE_PATH>",
      description = "Path to the TLS key store file",
      arity = "1")
  private Path tlsKeyStorePath = null;

  @Option(
      names = {TLS_KEY_STORE_PASSWORD},
      hidden = true,
      paramLabel = "<PASSWORD>",
      description = "TLS key store password",
      arity = "1")
  private String tlsKeyStorePassword = null;

  @Option(
      names = {TLS_TRUST_STORE_PATH},
      hidden = true,
      paramLabel = "<FILE_PATH>",
      description = "Path to the TLS trust store file",
      arity = "1")
  private Path tlsTrustStorePath = null;

  @Option(
      names = {TLS_TRUST_STORE_PASSWORD},
      hidden = true,
      paramLabel = "<PASSWORD>",
      description = "TLS trust store password",
      arity = "1")
  private String tlsTrustStorePassword = null;

  @Positive
  @Min(21000L)
  @Max(10000000L)
  @Option(
      names = {GAS_LIMIT},
      hidden = true,
      paramLabel = "<LONG>",
      description = "Gas limit for transactions (default: ${DEFAULT-VALUE})",
      arity = "1",
      defaultValue = "" + DEFAULT_GAS_LIMIT)
  private long gasLimit = DEFAULT_GAS_LIMIT;

  @Positive
  @Option(
      names = {GAS_PRICE},
      hidden = true,
      paramLabel = "<LONG>",
      description = "Gas price in Wei for transactions. (default: ${DEFAULT-VALUE})",
      arity = "1",
      defaultValue = "" + DEFAULT_GAS_PRICE)
  private long gasPrice = DEFAULT_GAS_PRICE;

  private LineaLivenessServiceCliOptions() {}

  /**
   * Create Linea liveness service cli options.
   *
   * @return the Linea liveness service cli options
   */
  public static LineaLivenessServiceCliOptions create() {
    return new LineaLivenessServiceCliOptions();
  }

  /**
   * Linea liveness Service CLI options from config.
   *
   * @param config the config
   * @return the Linea liveness service CLI options
   */
  public static LineaLivenessServiceCliOptions fromConfig(
      final LineaLivenessServiceCliOptions config) {
    final LineaLivenessServiceCliOptions options = create();
    options.enabled = config.enabled;
    options.maxBlockAgeSeconds = config.maxBlockAgeSeconds;
    options.bundleMaxTimestampSurplusSecond = config.bundleMaxTimestampSurplusSecond;
    options.contractAddress = config.contractAddress;
    options.signerUrl = config.signerUrl;
    options.signerKeyId = config.signerKeyId;
    options.signerAddress = config.signerAddress;
    options.tlsEnabled = config.tlsEnabled;
    options.tlsKeyStorePath = config.tlsKeyStorePath;
    options.tlsKeyStorePassword = config.tlsKeyStorePassword;
    options.tlsTrustStorePath = config.tlsTrustStorePath;
    options.tlsTrustStorePassword = config.tlsTrustStorePassword;
    options.gasLimit = config.gasLimit;
    options.gasPrice = config.gasPrice;
    return options;
  }

  /**
   * To domain object Linea factory configuration.
   *
   * @return the Linea factory configuration
   */
  @Override
  public LineaLivenessServiceConfiguration toDomainObject() {
    if (enabled) {
      if (contractAddress == null
          || signerUrl == null
          || signerKeyId == null
          || signerAddress == null) {
        throw new IllegalArgumentException(
            "Error: Missing some or all of these required argument(s) when liveness service is enabled: "
                + CONTRACT_ADDRESS
                + "=<SMC_ADDRESS>, "
                + SIGNER_URL
                + "=<URL>, "
                + SIGNER_KEY_ID
                + "=<PUBLIC_KEY_HEX>, "
                + SIGNER_ADDRESS
                + "=<EOA_ADDRESS>");
      }

      // Minimum gas limit for a contract call (21_000 for simple transfer plus some overhead)
      long minimumGasLimit = 21_000L;
      // Maximum reasonable gas limit (to prevent accidentally high values)
      long maximumGasLimit = 10_000_000L;

      if (gasLimit < minimumGasLimit || gasLimit > maximumGasLimit) {
        throw new IllegalArgumentException(
            "Error: "
                + GAS_LIMIT
                + " must be within ["
                + minimumGasLimit
                + ", "
                + maximumGasLimit
                + "] when liveness service is enabled");
      }

      if (tlsEnabled) {
        if (tlsKeyStorePath == null
            || tlsKeyStorePassword == null
            || tlsTrustStorePath == null
            || tlsTrustStorePassword == null) {
          throw new IllegalArgumentException(
              "Error: Missing some or all of these required argument(s) when TLS connection is enabled: "
                  + TLS_KEY_STORE_PATH
                  + "=<FILE_PATH>, "
                  + TLS_KEY_STORE_PASSWORD
                  + "=<PASSWORD>, "
                  + TLS_TRUST_STORE_PATH
                  + "=<FILE_PATH>, "
                  + TLS_TRUST_STORE_PASSWORD
                  + "=<PASSWORD>");
        }

        if (!signerUrl.toLowerCase().startsWith("https://")) {
          throw new IllegalArgumentException(
              "Error: " + SIGNER_URL + " needs to use HTTPS schema when TLS connection is enabled");
        }
      }
    }

    return LineaLivenessServiceConfiguration.builder()
        .enabled(enabled)
        .maxBlockAgeSeconds(Duration.ofSeconds(maxBlockAgeSeconds))
        .bundleMaxTimestampSurplusSecond(Duration.ofSeconds(bundleMaxTimestampSurplusSecond))
        .contractAddress(contractAddress)
        .signerUrl(signerUrl)
        .signerKeyId(signerKeyId)
        .signerAddress(signerAddress)
        .tlsEnabled(tlsEnabled)
        .tlsKeyStorePath(tlsKeyStorePath)
        .tlsKeyStorePassword(tlsKeyStorePassword)
        .tlsTrustStorePath(tlsTrustStorePath)
        .tlsTrustStorePassword(tlsTrustStorePassword)
        .gasLimit(gasLimit)
        .gasPrice(gasPrice)
        .build();
  }

  @Override
  public String toString() {
    return MoreObjects.toStringHelper(this)
        .add(ENABLED, enabled)
        .add(MAX_BLOCK_AGE_SECONDS, maxBlockAgeSeconds)
        .add(BUNDLE_MAX_TIMESTAMP_SURPLUS_SECONDS, bundleMaxTimestampSurplusSecond)
        .add(CONTRACT_ADDRESS, contractAddress)
        .add(SIGNER_URL, signerUrl)
        .add(SIGNER_KEY_ID, signerKeyId)
        .add(SIGNER_ADDRESS, signerAddress)
        .add(TLS_ENABLED, tlsEnabled)
        .add(TLS_KEY_STORE_PATH, tlsKeyStorePath)
        .add(TLS_KEY_STORE_PASSWORD, tlsKeyStorePassword)
        .add(TLS_TRUST_STORE_PATH, tlsTrustStorePath)
        .add(TLS_TRUST_STORE_PASSWORD, tlsTrustStorePassword)
        .add(GAS_LIMIT, gasLimit)
        .add(GAS_PRICE, gasPrice)
        .toString();
  }
}
