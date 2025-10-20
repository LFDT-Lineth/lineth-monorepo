/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */

package net.consensys.linea.rpc.services;

import com.google.auto.service.AutoService;
import java.util.Arrays;
import java.util.Collections;
import java.util.Optional;
import java.util.Set;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.atomic.AtomicReference;
import net.consensys.linea.AbstractLineaRequiredPlugin;
import net.consensys.linea.config.LineaTransactionPoolValidatorCliOptions;
import net.consensys.linea.rpc.methods.LineaCancelBundle;
import net.consensys.linea.rpc.methods.LineaSendBundle;
import net.consensys.linea.sequencer.txpoolvalidation.validators.AllowedAddressValidator;
import net.consensys.linea.sequencer.txpoolvalidation.validators.CalldataValidator;
import net.consensys.linea.sequencer.txpoolvalidation.validators.GasLimitValidator;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.plugin.BesuPlugin;
import org.hyperledger.besu.plugin.ServiceManager;
import org.hyperledger.besu.plugin.services.txvalidator.PluginTransactionPoolValidator;

@AutoService(BesuPlugin.class)
public class LineaBundleEndpointsPlugin extends AbstractLineaRequiredPlugin {
  private LineaSendBundle lineaSendBundleMethod;
  private LineaCancelBundle lineaCancelBundleMethod;

  private final AtomicReference<Set<Address>> bundleDeniedAddresses =
      new AtomicReference<>(Collections.emptySet());

  /**
   * Register the bundle RPC service.
   *
   * @param serviceManager the ServiceManager to be used.
   */
  @Override
  public void doRegister(final ServiceManager serviceManager) {
    lineaSendBundleMethod = new LineaSendBundle(blockchainService);

    rpcEndpointService.registerRPCEndpoint(
        lineaSendBundleMethod.getNamespace(),
        lineaSendBundleMethod.getName(),
        lineaSendBundleMethod::execute);

    lineaCancelBundleMethod = new LineaCancelBundle();

    rpcEndpointService.registerRPCEndpoint(
        lineaCancelBundleMethod.getNamespace(),
        lineaCancelBundleMethod.getName(),
        lineaCancelBundleMethod::execute);
  }

  public PluginTransactionPoolValidator createTransactionValidator() {
    final var validators =
        new PluginTransactionPoolValidator[] {
          new AllowedAddressValidator(bundleDeniedAddresses),
          new GasLimitValidator(transactionPoolValidatorConfiguration().maxTxGasLimit()),
          new CalldataValidator(transactionPoolValidatorConfiguration().maxTxCalldataSize())
        };

    return (transaction, isLocal, hasPriority) ->
        Arrays.stream(validators)
            .map(v -> v.validateTransaction(transaction, isLocal, hasPriority))
            .filter(Optional::isPresent)
            .findFirst()
            .map(Optional::get);
  }

  /**
   * Starts this plugin and in case the extra data pricing is enabled, as first thing it tries to
   * extract extra data pricing configuration from the chain head, then it starts listening for new
   * imported block, in order to update the extra data pricing on every incoming block.
   */
  @Override
  public void doStart() {
    // set the pool
    lineaSendBundleMethod.init(bundlePoolService, createTransactionValidator());
    lineaCancelBundleMethod.init(bundlePoolService);

    bundleDeniedAddresses.set(transactionPoolValidatorConfiguration().bundleDeniedAddresses());
  }

  @Override
  public CompletableFuture<Void> reloadConfiguration() {
    try {
      bundleDeniedAddresses.set(
          LineaTransactionPoolValidatorCliOptions.create()
              .parseDeniedAddresses(
                  transactionPoolValidatorConfiguration().bundleOverridingDenyListPath()));
      return CompletableFuture.completedFuture(null);
    } catch (Exception e) {
      return CompletableFuture.failedFuture(e);
    }
  }

  @Override
  public void stop() {
    bundlePoolService.saveToDisk();
    super.stop();
  }
}
