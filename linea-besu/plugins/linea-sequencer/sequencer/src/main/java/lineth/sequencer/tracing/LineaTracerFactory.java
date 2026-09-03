/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package lineth.sequencer.tracing;

import static net.consensys.linea.zktracer.Fork.fromMainnetHardforkIdToTracerFork;

import java.math.BigInteger;
import java.util.Optional;
import java.util.function.LongFunction;
import java.util.function.Supplier;
import lineth.config.LineaTracerConfiguration;
import net.consensys.linea.plugins.config.LineaL1L2BridgeSharedConfiguration;
import net.consensys.linea.zktracer.Fork;
import net.consensys.linea.zktracer.LineCountingTracer;
import net.consensys.linea.zktracer.ZkCounter;
import net.consensys.linea.zktracer.ZkTracer;
import org.hyperledger.besu.datatypes.HardforkId;
import org.hyperledger.besu.plugin.services.BlockchainService;

/** Creates sequencer bespoke tracers while their activation policy is enabled. */
public class LineaTracerFactory {
  private final BespokeTracingActivationPolicy activationPolicy;
  private final LineaTracerConfiguration tracerConfiguration;
  private final LineaL1L2BridgeSharedConfiguration bridgeConfiguration;
  private final Supplier<BigInteger> chainIdSupplier;
  private final LongFunction<Fork> hardforkResolver;

  public LineaTracerFactory(
      final BespokeTracingActivationPolicy activationPolicy,
      final LineaTracerConfiguration tracerConfiguration,
      final LineaL1L2BridgeSharedConfiguration bridgeConfiguration,
      final Supplier<BigInteger> chainIdSupplier,
      final LongFunction<Fork> hardforkResolver) {
    this.activationPolicy = activationPolicy;
    this.tracerConfiguration = tracerConfiguration;
    this.bridgeConfiguration = bridgeConfiguration;
    this.chainIdSupplier = chainIdSupplier;
    this.hardforkResolver = hardforkResolver;
  }

  public static LineaTracerFactory fromBlockchainService(
      final BlockchainService blockchainService,
      final LineaTracerConfiguration tracerConfiguration,
      final LineaL1L2BridgeSharedConfiguration bridgeConfiguration) {
    return new LineaTracerFactory(
        new BespokeTracingActivationPolicy(tracerConfiguration.tracingEndTimestamp()),
        tracerConfiguration,
        bridgeConfiguration,
        () ->
            blockchainService
                .getChainId()
                .orElseThrow(
                    () ->
                        new IllegalStateException("Failed to get chain ID from BlockchainService")),
        pendingBlockTimestamp ->
            fromMainnetHardforkIdToTracerFork(
                (HardforkId.MainnetHardforkId)
                    blockchainService.getNextBlockHardforkId(
                        blockchainService.getChainHeadHeader(), pendingBlockTimestamp)));
  }

  public Optional<LineCountingTracer> create(final long pendingBlockTimestamp) {
    if (!activationPolicy.shouldTrace(pendingBlockTimestamp)) {
      return Optional.empty();
    }

    final Fork fork = hardforkResolver.apply(pendingBlockTimestamp);
    return Optional.of(
        tracerConfiguration.isLimitless()
            ? new ZkCounter(bridgeConfiguration, fork)
            : new ZkTracer(fork, bridgeConfiguration, chainIdSupplier.get()));
  }
}
