/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */

package lineth.sequencer.txvalidation;

import com.google.auto.service.AutoService;
import java.util.concurrent.atomic.AtomicBoolean;
import lineth.AbstractLineaRequiredPlugin;
import lineth.config.LineaTransactionValidatorConfiguration;
import lombok.extern.slf4j.Slf4j;
import org.hyperledger.besu.plugin.BesuPlugin;
import org.hyperledger.besu.plugin.ServiceManager;
import org.hyperledger.besu.plugin.services.TransactionValidatorService;

/**
 * Registers protocol-level transaction validation rules via {@link TransactionValidatorService}.
 * These rules apply during block import and transaction selection, enforcing which transaction
 * types (e.g. blob, delegate code) are accepted at the protocol level.
 *
 * <p>Note: Besu's {@link TransactionValidatorService} rules also run during transaction pool
 * admission (RPC/P2P), so the type checks here overlap with the pool-level type validation in
 * {@link lineth.sequencer.txpoolvalidation.LineaTransactionPoolValidatorPlugin}. The two plugins
 * may be enabled together; to avoid validating transaction types twice at pool admission, the pool
 * plugin skips its own type validator when this plugin is registered (detected via {@link
 * #registered}). This plugin remains the source of type enforcement during block import and block
 * production.
 */
@Slf4j
@AutoService(BesuPlugin.class)
public class LineaBlockTransactionValidatorPlugin extends AbstractLineaRequiredPlugin {

  /**
   * {@code true} while this plugin is registered and running; reset to {@code false} in {@link
   * #stop()} so that a plugin restart cycle leaves the flag in the correct state.
   *
   * <p>Used by {@link lineth.sequencer.txpoolvalidation.LineaTransactionPoolValidatorPlugin} to
   * detect that protocol-level transaction-type validation is active and skip its own redundant
   * pool-level type validator, since this plugin's rule already runs at pool admission.
   */
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
