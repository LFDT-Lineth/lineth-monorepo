/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package lineth.sequencer.txpoolvalidation.validators;

import static org.assertj.core.api.Assertions.assertThat;

import java.util.OptionalLong;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.function.LongSupplier;
import lineth.sequencer.tracing.BespokeTracingActivationPolicy;
import lineth.sequencer.txselection.InvalidTransactionByLineCountCache;
import org.hyperledger.besu.crypto.SignatureAlgorithmFactory;
import org.hyperledger.besu.datatypes.Transaction;
import org.hyperledger.besu.ethereum.core.TransactionTestFixture;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

class TraceLineLimitValidatorTest {
  private InvalidTransactionByLineCountCache cache;
  private Transaction transaction;

  @BeforeEach
  void setUp() {
    cache = new InvalidTransactionByLineCountCache(10);
    transaction =
        new TransactionTestFixture()
            .nonce(0L)
            .gasLimit(21_000L)
            .createTransaction(SignatureAlgorithmFactory.getInstance().generateKeyPair());
  }

  @Test
  void shouldNotRequestPendingTimestampForCacheMiss() {
    final var timestampRequests = new AtomicInteger();
    final var validator = validator(100L, timestampRequests::incrementAndGet);

    assertThat(validator.validateTransaction(transaction, true, false)).isEmpty();
    assertThat(timestampRequests).hasValue(0);
  }

  @Test
  void shouldRejectCachedTransactionBeforeCutoff() {
    cache.remember(transaction.getHash(), "EXT");
    final var validator = validator(100L, () -> 99L);

    assertThat(validator.validateTransaction(transaction, true, false))
        .hasValueSatisfying(
            reason ->
                assertThat(reason)
                    .contains("was already identified to go over line count limit")
                    .contains(transaction.getHash().getBytes().toHexString()));
  }

  @Test
  void shouldAcceptCachedTransactionAtCutoff() {
    cache.remember(transaction.getHash(), "EXT");
    final var validator = validator(100L, () -> 100L);

    assertThat(validator.validateTransaction(transaction, true, false)).isEmpty();
  }

  @Test
  void shouldRejectCachedTransactionWhenPendingTimestampIsUnavailable() {
    cache.remember(transaction.getHash(), "EXT");
    final var validator =
        validator(
            100L,
            () -> {
              throw new IllegalStateException("pending header unavailable");
            });

    assertThat(validator.validateTransaction(transaction, true, false)).isPresent();
  }

  private TraceLineLimitValidator validator(
      final long tracingEndTimestamp, final LongSupplier pendingBlockTimestampSupplier) {
    return new TraceLineLimitValidator(
        cache,
        new BespokeTracingActivationPolicy(OptionalLong.of(tracingEndTimestamp)),
        pendingBlockTimestampSupplier);
  }
}
