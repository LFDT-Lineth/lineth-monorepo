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

import java.util.Optional;
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

public class TraceLineLimitValidatorTest {
  private InvalidTransactionByLineCountCache cache;

  @BeforeEach
  void setUp() {
    cache = new InvalidTransactionByLineCountCache(10);
  }

  @Test
  void shouldAcceptTransactionNotInCache() {
    final var transaction = realTransaction();

    assertThat(cache.contains(transaction.getHash())).isFalse();
    assertThat(validatorWithoutCutoff().validateTransaction(transaction, true, false)).isEmpty();
  }

  @Test
  void shouldRejectTransactionInCache() {
    final var transaction = realTransaction();
    cache.remember(transaction.getHash(), "EXT");

    final Optional<String> result =
        validatorWithoutCutoff().validateTransaction(transaction, true, false);

    assertThat(cache.contains(transaction.getHash())).isTrue();
    assertThat(result).isPresent();
    assertThat(result.get()).contains("was already identified to go over line count limit");
    assertThat(result.get()).contains(transaction.getHash().getBytes().toHexString());
  }

  @Test
  void shouldRejectTransactionInCache_LocalTransaction() {
    final var transaction = realTransaction();
    cache.remember(transaction.getHash(), "EXT");

    assertThat(validatorWithoutCutoff().validateTransaction(transaction, true, false)).isPresent();
  }

  @Test
  void shouldRejectTransactionInCache_RemoteTransaction() {
    final var transaction = realTransaction();
    cache.remember(transaction.getHash(), "EXT");

    assertThat(validatorWithoutCutoff().validateTransaction(transaction, false, false)).isPresent();
  }

  @Test
  void shouldRejectTransactionInCache_PriorityTransaction() {
    final var transaction = realTransaction();
    cache.remember(transaction.getHash(), "EXT");

    assertThat(validatorWithoutCutoff().validateTransaction(transaction, false, true)).isPresent();
  }

  @Test
  void shouldHandleMultipleTransactionsCorrectly() {
    final var transaction1 = realTransaction(1L);
    final var transaction2 = realTransaction(2L);
    final var transaction3 = realTransaction(3L);
    cache.remember(transaction1.getHash(), "EXT");
    cache.remember(transaction3.getHash(), "RAM");
    final var validator = validatorWithoutCutoff();

    assertThat(validator.validateTransaction(transaction1, true, false)).isPresent();
    assertThat(validator.validateTransaction(transaction2, true, false)).isEmpty();
    assertThat(validator.validateTransaction(transaction3, true, false)).isPresent();
  }

  @Test
  void shouldNotRequestPendingTimestampForCacheMiss() {
    final var timestampRequests = new AtomicInteger();
    final var validator = cutoffValidator(100L, timestampRequests::incrementAndGet);

    assertThat(validator.validateTransaction(realTransaction(), true, false)).isEmpty();
    assertThat(timestampRequests).hasValue(0);
  }

  @Test
  void shouldRejectCachedTransactionBeforeCutoff() {
    final var transaction = realTransaction();
    cache.remember(transaction.getHash(), "EXT");
    final var validator = cutoffValidator(100L, () -> 99L);

    assertThat(validator.validateTransaction(transaction, true, false))
        .hasValueSatisfying(
            reason ->
                assertThat(reason)
                    .contains("was already identified to go over line count limit")
                    .contains(transaction.getHash().getBytes().toHexString()));
  }

  @Test
  void shouldAcceptCachedTransactionAtCutoff() {
    final var transaction = realTransaction();
    cache.remember(transaction.getHash(), "EXT");
    final var validator = cutoffValidator(100L, () -> 100L);

    assertThat(validator.validateTransaction(transaction, true, false)).isEmpty();
  }

  @Test
  void shouldRejectCachedTransactionWhenPendingTimestampIsUnavailable() {
    final var transaction = realTransaction();
    cache.remember(transaction.getHash(), "EXT");
    final var validator =
        cutoffValidator(
            100L,
            () -> {
              throw new IllegalStateException("pending header unavailable");
            });

    assertThat(validator.validateTransaction(transaction, true, false)).isPresent();
  }

  private Transaction realTransaction() {
    return realTransaction(0L);
  }

  private Transaction realTransaction(final long nonce) {
    return new TransactionTestFixture()
        .nonce(nonce)
        .gasLimit(21_000L)
        .createTransaction(SignatureAlgorithmFactory.getInstance().generateKeyPair());
  }

  private TraceLineLimitValidator validatorWithoutCutoff() {
    return new TraceLineLimitValidator(
        cache, new BespokeTracingActivationPolicy(OptionalLong.empty()), () -> 0L);
  }

  private TraceLineLimitValidator cutoffValidator(
      final long tracingEndTimestamp, final LongSupplier pendingBlockTimestampSupplier) {
    return new TraceLineLimitValidator(
        cache,
        new BespokeTracingActivationPolicy(OptionalLong.of(tracingEndTimestamp)),
        pendingBlockTimestampSupplier);
  }
}
