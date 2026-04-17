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
import java.util.concurrent.atomic.AtomicBoolean;
import lombok.extern.slf4j.Slf4j;
import net.consensys.linea.AbstractLineaRequiredPlugin;
import net.consensys.linea.config.LineaTransactionValidatorConfiguration;
import net.consensys.linea.sequencer.txpoolvalidation.LineaTransactionPoolValidatorPlugin;
import org.hyperledger.besu.plugin.BesuPlugin;
import org.hyperledger.besu.plugin.ServiceManager;
import org.hyperledger.besu.plugin.services.TransactionValidatorService;

/**
 * Registers protocol-level transaction validation rules via {@link TransactionValidatorService}.
 * These rules apply during block import and transaction selection, enforcing which transaction
 * types (e.g. blob, delegate code) are accepted at the protocol level.
 *
 * <p>Note: Besu's {@link TransactionValidatorService} rules also run during transaction pool
 * admission (RPC/P2P). Pool-level type validation is explicitly handled by {@link
 * net.consensys.linea.sequencer.txpoolvalidation.LineaTransactionPoolValidatorPlugin}.
 */
@Slf4j
@AutoService(BesuPlugin.class)
public class LineaBlockTransactionValidatorPlugin extends AbstractLineaRequiredPlugin {
  public static final AtomicBoolean registered = new AtomicBoolean(false);

  private TransactionValidatorService transactionValidatorService;
  private LineaTransactionValidatorConfiguration config;

  @Override
  public void doRegister(final ServiceManager serviceManager) {
    registered.set(true);
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
    if (LineaTransactionPoolValidatorPlugin.registered.get()) {
      throw new IllegalStateException(
          "Both LineaBlockTransactionValidatorPlugin and LineaTransactionPoolValidatorPlugin are"
              + " enabled. Only one should be active at a time since their transaction type"
              + " validation functionality overlaps. Use LineaTransactionPoolValidatorPlugin for"
              + " RPC/P2P nodes or LineaBlockTransactionValidatorPlugin for validator nodes.");
    }
    this.config = transactionValidatorConfiguration();
    this.transactionValidatorService.registerTransactionValidatorRule(
        (tx) -> TransactionTypeValidation.validate(tx, config));
  }

  @Override
  public void doStart() {}

  @Override
  public void stop() {
    super.stop();
    registered.set(false);
  }
}
