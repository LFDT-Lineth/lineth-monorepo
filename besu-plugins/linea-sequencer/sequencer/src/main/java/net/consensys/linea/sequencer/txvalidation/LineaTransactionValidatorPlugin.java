/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */

package net.consensys.linea.sequencer.txvalidation;

import com.google.auto.service.AutoService;
import java.util.Optional;
import lombok.extern.slf4j.Slf4j;
import net.consensys.linea.AbstractLineaRequiredPlugin;
import net.consensys.linea.config.LineaTransactionValidatorConfiguration;
import org.hyperledger.besu.datatypes.TransactionType;
import org.hyperledger.besu.plugin.BesuPlugin;
import org.hyperledger.besu.plugin.ServiceManager;
import org.hyperledger.besu.plugin.services.TransactionValidatorService;

/**
 * This class extends the default transaction validation rules for adding transactions to the
 * transaction pool. It leverages the PluginTransactionValidatorService to manage and customize the
 * process of transaction validation. This includes, for example, setting a deny list of addresses
 * that are not allowed to add transactions to the pool.
 */
@Slf4j
@AutoService(BesuPlugin.class)
public class LineaTransactionValidatorPlugin extends AbstractLineaRequiredPlugin {
  private TransactionValidatorService transactionValidatorService;
  private LineaTransactionValidatorConfiguration config;

  public enum LineaTransactionValidatorError {
    BLOB_TX_NOT_ALLOWED;

    @Override
    public String toString() {
      return "LineaTransactionValidatorPlugin - " + name();
    }
  }

  @Override
  public void doRegister(final ServiceManager serviceManager) {
    transactionValidatorService =
        serviceManager
            .getService(TransactionValidatorService.class)
            .orElseThrow(
                () ->
                    new RuntimeException(
                        "Failed to obtain TransactionValidatorService from the ServiceManager."));
  }

  // CLI config is not available in doRegister
  // 'registerTransactionValidatorRule' does not do anything if done in doStart
  // Therefore we must use beforeExternalServices hook
  @Override
  public void beforeExternalServices() {
    super.beforeExternalServices();
    this.config = transactionValidatorConfiguration();
    // Register rule to reject blob transactions
    this.transactionValidatorService.registerTransactionValidatorRule(
        (tx) -> {
          if (tx.getType() == TransactionType.BLOB && !config.blobTxEnabled())
            return Optional.of(LineaTransactionValidatorError.BLOB_TX_NOT_ALLOWED.toString());
          return Optional.empty();
        });
  }

  @Override
  public void doStart() {}

  @Override
  public void stop() {
    super.stop();
  }
}
