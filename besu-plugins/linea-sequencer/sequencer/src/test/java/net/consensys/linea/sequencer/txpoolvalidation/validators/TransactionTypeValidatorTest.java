/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */

package net.consensys.linea.sequencer.txpoolvalidation.validators;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.Mockito.when;

import java.util.Optional;
import net.consensys.linea.config.LineaTransactionValidatorConfiguration;
import net.consensys.linea.sequencer.txvalidation.TransactionTypeValidation;
import org.hyperledger.besu.datatypes.TransactionType;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;

@ExtendWith(MockitoExtension.class)
public class TransactionTypeValidatorTest {

  @Mock private Transaction transaction;

  @Test
  public void shouldRejectBlobTransactionWhenDisabled() {
    final var config = new LineaTransactionValidatorConfiguration(false, true);
    final var validator = new TransactionTypeValidator(config);

    when(transaction.getType()).thenReturn(TransactionType.BLOB);
    final Optional<String> result = validator.validateTransaction(transaction, false, false);

    assertThat(result).isPresent();
    assertThat(result.get())
        .isEqualTo(TransactionTypeValidation.Error.BLOB_TX_NOT_ALLOWED.toString());
  }

  @Test
  public void shouldAcceptBlobTransactionWhenEnabled() {
    final var config = new LineaTransactionValidatorConfiguration(true, true);
    final var validator = new TransactionTypeValidator(config);

    when(transaction.getType()).thenReturn(TransactionType.BLOB);
    final Optional<String> result = validator.validateTransaction(transaction, false, false);

    assertThat(result).isEmpty();
  }

  @Test
  public void shouldRejectDelegateCodeTransactionWhenDisabled() {
    final var config = new LineaTransactionValidatorConfiguration(false, false);
    final var validator = new TransactionTypeValidator(config);

    when(transaction.getType()).thenReturn(TransactionType.DELEGATE_CODE);
    final Optional<String> result = validator.validateTransaction(transaction, false, false);

    assertThat(result).isPresent();
    assertThat(result.get())
        .isEqualTo(TransactionTypeValidation.Error.DELEGATE_CODE_TX_NOT_ALLOWED.toString());
  }

  @Test
  public void shouldAcceptDelegateCodeTransactionWhenEnabled() {
    final var config = new LineaTransactionValidatorConfiguration(false, true);
    final var validator = new TransactionTypeValidator(config);

    when(transaction.getType()).thenReturn(TransactionType.DELEGATE_CODE);
    final Optional<String> result = validator.validateTransaction(transaction, false, false);

    assertThat(result).isEmpty();
  }

  @Test
  public void shouldAcceptFrontierTransaction() {
    final var config = new LineaTransactionValidatorConfiguration(false, false);
    final var validator = new TransactionTypeValidator(config);

    when(transaction.getType()).thenReturn(TransactionType.FRONTIER);
    final Optional<String> result = validator.validateTransaction(transaction, false, false);

    assertThat(result).isEmpty();
  }

  @Test
  public void shouldAcceptEIP1559Transaction() {
    final var config = new LineaTransactionValidatorConfiguration(false, false);
    final var validator = new TransactionTypeValidator(config);

    when(transaction.getType()).thenReturn(TransactionType.EIP1559);
    final Optional<String> result = validator.validateTransaction(transaction, false, false);

    assertThat(result).isEmpty();
  }

  @Test
  public void shouldAcceptAccessListTransaction() {
    final var config = new LineaTransactionValidatorConfiguration(false, false);
    final var validator = new TransactionTypeValidator(config);

    when(transaction.getType()).thenReturn(TransactionType.ACCESS_LIST);
    final Optional<String> result = validator.validateTransaction(transaction, false, false);

    assertThat(result).isEmpty();
  }
}
