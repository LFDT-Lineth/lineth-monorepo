/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package lineth.sequencer.tracing;

import static org.assertj.core.api.Assertions.assertThat;

import java.math.BigInteger;
import java.util.Map;
import java.util.OptionalLong;
import java.util.concurrent.atomic.AtomicLong;
import java.util.concurrent.atomic.AtomicReference;
import java.util.function.LongFunction;
import java.util.function.Supplier;
import lineth.config.LineaTracerConfiguration;
import net.consensys.linea.plugins.config.LineaL1L2BridgeSharedConfiguration;
import net.consensys.linea.zktracer.Fork;
import net.consensys.linea.zktracer.ZkCounter;
import net.consensys.linea.zktracer.ZkTracer;
import org.apache.tuweni.bytes.Bytes32;
import org.hyperledger.besu.datatypes.Address;
import org.junit.jupiter.api.Test;

class LineaTracerFactoryTest {
  private static final LineaL1L2BridgeSharedConfiguration BRIDGE_CONFIGURATION =
      LineaL1L2BridgeSharedConfiguration.builder()
          .contract(Address.fromHexString("0xdeadbeef"))
          .topic(Bytes32.fromHexString("0xc0ffee"))
          .build();

  @Test
  void shouldResolveHardforkUsingPendingBlockTimestampBeforeCutoff() {
    final AtomicLong resolvedPendingBlockTimestamp = new AtomicLong(-1L);
    final var factory =
        factory(
            OptionalLong.of(100L),
            true,
            () -> BigInteger.ONE,
            pendingBlockTimestamp -> {
              resolvedPendingBlockTimestamp.set(pendingBlockTimestamp);
              return Fork.OSAKA;
            });

    assertThat(factory.create(99L)).isPresent();
    assertThat(resolvedPendingBlockTimestamp).hasValue(99L);
  }

  @Test
  void shouldNotResolveTracerDependenciesAtOrAfterCutoff() {
    final var factory =
        factory(
            OptionalLong.of(100L),
            true,
            () -> {
              throw new AssertionError("Chain ID must not be resolved after cutoff");
            },
            pendingBlockTimestamp -> {
              throw new AssertionError("Hardfork resolver must not be called after cutoff");
            });

    assertThat(factory.create(100L)).isEmpty();
    assertThat(factory.create(101L)).isEmpty();
  }

  @Test
  void shouldNotResolveChainIdForLimitlessTracer() {
    final var factory =
        factory(
            OptionalLong.empty(),
            true,
            () -> {
              throw new AssertionError("Chain ID must not be resolved for ZkCounter");
            },
            pendingBlockTimestamp -> Fork.OSAKA);

    assertThat(factory.create(0L))
        .hasValueSatisfying(tracer -> assertThat(tracer).isInstanceOf(ZkCounter.class));
  }

  @Test
  void shouldResolveChainIdWhenCreatingZkTracer() {
    final AtomicReference<BigInteger> resolvedChainId = new AtomicReference<>();
    final Supplier<BigInteger> chainIdSupplier =
        () -> {
          final BigInteger chainId = BigInteger.valueOf(59144L);
          resolvedChainId.set(chainId);
          return chainId;
        };
    final var factory =
        factory(OptionalLong.empty(), false, chainIdSupplier, pendingBlockTimestamp -> Fork.OSAKA);

    assertThat(factory.create(0L))
        .hasValueSatisfying(tracer -> assertThat(tracer).isInstanceOf(ZkTracer.class));
    assertThat(resolvedChainId).hasValue(BigInteger.valueOf(59144L));
  }

  private LineaTracerFactory factory(
      final OptionalLong tracingEndTimestamp,
      final boolean limitless,
      final Supplier<BigInteger> chainIdSupplier,
      final LongFunction<Fork> hardforkResolver) {
    final var configuration =
        LineaTracerConfiguration.builder()
            .moduleLimitsFilePath("unused.toml")
            .moduleLimitsMap(Map.of())
            .isLimitless(limitless)
            .tracingEndTimestamp(tracingEndTimestamp)
            .build();
    return new LineaTracerFactory(
        new BespokeTracingActivationPolicy(tracingEndTimestamp),
        configuration,
        BRIDGE_CONFIGURATION,
        chainIdSupplier,
        hardforkResolver);
  }
}
