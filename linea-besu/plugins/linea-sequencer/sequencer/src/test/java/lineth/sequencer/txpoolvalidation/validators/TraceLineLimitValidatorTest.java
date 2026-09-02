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
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

import java.util.Optional;
import java.util.OptionalLong;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.function.LongSupplier;
import lineth.sequencer.tracing.BespokeTracingActivationPolicy;
import lineth.sequencer.txselection.InvalidTransactionByLineCountCache;
import org.apache.tuweni.bytes.Bytes32;
import org.hyperledger.besu.crypto.SignatureAlgorithmFactory;
import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.datatypes.Transaction;
import org.hyperledger.besu.ethereum.core.TransactionTestFixture;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Nested;
import org.junit.jupiter.api.Test;

public class TraceLineLimitValidatorTest {
  private InvalidTransactionByLineCountCache cache;

  @BeforeEach
  void setUp() {
    cache = new InvalidTransactionByLineCountCache(10);
  }

  @Nested
  class ExistingBehaviorTests {
    private TraceLineLimitValidator validator;
    private Transaction mockTransaction;
    private Hash transactionHash;

    @BeforeEach
    void setUpExistingBehavior() {
      validator =
          new TraceLineLimitValidator(
              cache, new BespokeTracingActivationPolicy(OptionalLong.empty()), () -> 0L);
      mockTransaction = mock(Transaction.class);
      transactionHash = Hash.wrap(Bytes32.random());
      when(mockTransaction.getHash()).thenReturn(transactionHash);
    }

    @Test
    void shouldAcceptTransactionNotInCache() {
      // Given: transaction is not in cache
      assertThat(cache.contains(transactionHash)).isFalse();

      // When: validating transaction
      Optional<String> result = validator.validateTransaction(mockTransaction, true, false);

      // Then: transaction should be accepted (no validation error)
      assertThat(result).isEmpty();
    }

    @Test
    void shouldRejectTransactionInCache() {
      // Given: transaction is in cache (marked as exceeding line count limit)
      cache.remember(transactionHash, "EXT");
      assertThat(cache.contains(transactionHash)).isTrue();

      // When: validating transaction
      Optional<String> result = validator.validateTransaction(mockTransaction, true, false);

      // Then: transaction should be rejected with appropriate error message
      assertThat(result).isPresent();
      assertThat(result.get()).contains("was already identified to go over line count limit");
      assertThat(result.get()).contains(transactionHash.getBytes().toHexString());
    }

    @Test
    void shouldRejectTransactionInCache_LocalTransaction() {
      // Given: transaction is in cache
      cache.remember(transactionHash, "EXT");

      // When: validating local transaction
      Optional<String> result = validator.validateTransaction(mockTransaction, true, false);

      // Then: transaction should be rejected regardless of local status
      assertThat(result).isPresent();
    }

    @Test
    void shouldRejectTransactionInCache_RemoteTransaction() {
      // Given: transaction is in cache
      cache.remember(transactionHash, "EXT");

      // When: validating remote transaction
      Optional<String> result = validator.validateTransaction(mockTransaction, false, false);

      // Then: transaction should be rejected regardless of remote status
      assertThat(result).isPresent();
    }

    @Test
    void shouldRejectTransactionInCache_PriorityTransaction() {
      // Given: transaction is in cache
      cache.remember(transactionHash, "EXT");

      // When: validating priority transaction
      Optional<String> result = validator.validateTransaction(mockTransaction, false, true);

      // Then: transaction should be rejected regardless of priority status
      assertThat(result).isPresent();
    }

    @Test
    void shouldHandleMultipleTransactionsCorrectly() {
      // Given: multiple transactions, some in cache
      Hash hash1 = Hash.wrap(Bytes32.random());
      Hash hash2 = Hash.wrap(Bytes32.random());
      Hash hash3 = Hash.wrap(Bytes32.random());

      Transaction tx1 = mock(Transaction.class);
      Transaction tx2 = mock(Transaction.class);
      Transaction tx3 = mock(Transaction.class);

      when(tx1.getHash()).thenReturn(hash1);
      when(tx2.getHash()).thenReturn(hash2);
      when(tx3.getHash()).thenReturn(hash3);

      // Add only tx1 and tx3 to cache
      cache.remember(hash1, "EXT");
      cache.remember(hash3, "RAM");

      // When: validating all transactions
      Optional<String> result1 = validator.validateTransaction(tx1, true, false);
      Optional<String> result2 = validator.validateTransaction(tx2, true, false);
      Optional<String> result3 = validator.validateTransaction(tx3, true, false);

      // Then: only cached transactions should be rejected
      assertThat(result1).isPresent(); // tx1 is in cache - rejected
      assertThat(result2).isEmpty(); // tx2 is not in cache - accepted
      assertThat(result3).isPresent(); // tx3 is in cache - rejected
    }
  }

  @Nested
  class TracingCutoffTests {
    private Transaction transaction;

    @BeforeEach
    void setUpTransaction() {
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
  }

  private TraceLineLimitValidator validator(
      final long tracingEndTimestamp, final LongSupplier pendingBlockTimestampSupplier) {
    return new TraceLineLimitValidator(
        cache,
        new BespokeTracingActivationPolicy(OptionalLong.of(tracingEndTimestamp)),
        pendingBlockTimestampSupplier);
  }
}
