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
import net.consensys.linea.plugins.LineaCliOptions;
import picocli.CommandLine;

/** CLI options specific to the LineaTransactionValidator Plugin. */
public class LineaTransactionValidatorCliOptions implements LineaCliOptions {
  public static final String CONFIG_KEY = "transaction-validator-config";

  public static final String BLOB_TX_ENABLED = "--plugin-linea-blob-tx-enabled";
  public static final boolean DEFAULT_BLOB_TX_ENABLED = false;

  public static final String DELEGATE_CODE_TX_ENABLED = "--plugin-linea-delegate-code-tx-enabled";
  public static final boolean DEFAULT_DELEGATE_CODE_TX_ENABLED = false;

  @CommandLine.Option(
      names = {BLOB_TX_ENABLED},
      arity = "0..1",
      hidden = true,
      paramLabel = "<BOOLEAN>",
      description = "Enable blob transactions? (default: ${DEFAULT-VALUE})")
  private boolean blobTxEnabled = DEFAULT_BLOB_TX_ENABLED;

  @CommandLine.Option(
      names = {DELEGATE_CODE_TX_ENABLED},
      arity = "0..1",
      hidden = true,
      paramLabel = "<BOOLEAN>",
      description = "Enable EIP7702 delegate code transactions? (default: ${DEFAULT-VALUE})")
  private boolean delegateCodeTxEnabled = DEFAULT_DELEGATE_CODE_TX_ENABLED;

  public LineaTransactionValidatorCliOptions() {}

  /**
   * Create Linea cli options.
   *
   * @return the Linea cli options
   */
  public static LineaTransactionValidatorCliOptions create() {
    return new LineaTransactionValidatorCliOptions();
  }

  /**
   * Cli options from config.
   *
   * @param config the config
   * @return the cli options
   */
  public static LineaTransactionValidatorCliOptions fromConfig(
      final LineaTransactionValidatorConfiguration config) {
    final LineaTransactionValidatorCliOptions options = create();
    options.blobTxEnabled = config.blobTxEnabled();
    options.delegateCodeTxEnabled = config.delegateCodeTxEnabled();
    return options;
  }

  /**
   * To domain object Linea factory configuration.
   *
   * @return the Linea factory configuration
   */
  @Override
  public LineaTransactionValidatorConfiguration toDomainObject() {
    return new LineaTransactionValidatorConfiguration(blobTxEnabled, delegateCodeTxEnabled);
  }

  @Override
  public String toString() {
    return MoreObjects.toStringHelper(this)
        .add(BLOB_TX_ENABLED, blobTxEnabled)
        .add(DELEGATE_CODE_TX_ENABLED, delegateCodeTxEnabled)
        .toString();
  }
}
