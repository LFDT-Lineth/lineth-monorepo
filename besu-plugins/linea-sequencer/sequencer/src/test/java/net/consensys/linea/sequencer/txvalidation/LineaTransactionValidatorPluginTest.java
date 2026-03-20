/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package net.consensys.linea.sequencer.txvalidation;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

import java.util.Optional;
import net.consensys.linea.config.LineaTransactionValidatorConfiguration;
import net.consensys.linea.sequencer.txvalidation.LineaTransactionValidatorPlugin.LineaTransactionValidatorError;
import org.hyperledger.besu.datatypes.TransactionType;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.hyperledger.besu.plugin.ServiceManager;
import org.hyperledger.besu.plugin.services.TransactionValidatorService;
import org.hyperledger.besu.plugin.services.txvalidator.TransactionValidationRule;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.ArgumentCaptor;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;

@ExtendWith(MockitoExtension.class)
public class LineaTransactionValidatorPluginTest {

  @Mock private ServiceManager serviceManager;
  @Mock private TransactionValidatorService transactionValidatorService;
  @Mock private Transaction transaction;
  @Mock private LineaTransactionValidatorConfiguration lineaTransactionValidatorConfiguration;

  private LineaTransactionValidatorPlugin plugin;

  @BeforeEach
  public void setUp() {
    plugin =
        new LineaTransactionValidatorPlugin() {
          @Override
          public LineaTransactionValidatorConfiguration transactionValidatorConfiguration() {
            return lineaTransactionValidatorConfiguration;
          }
        };
    when(serviceManager.getService(TransactionValidatorService.class))
        .thenReturn(Optional.of(transactionValidatorService));
  }

  @Test
  public void shouldRegisterWithServiceManager() {
    // Act
    plugin.doRegister(serviceManager);

    // Assert
    verify(serviceManager).getService(TransactionValidatorService.class);
  }

  @Test
  public void shouldThrowExceptionWhenTransactionValidatorServiceNotAvailable() {
    // Arrange
    when(serviceManager.getService(TransactionValidatorService.class)).thenReturn(Optional.empty());

    // Act/Assert
    assertThatThrownBy(() -> plugin.doRegister(serviceManager))
        .isInstanceOf(RuntimeException.class)
        .hasMessageContaining(
            "Failed to obtain TransactionValidatorService from the ServiceManager");
  }

  @Test
  public void shouldRegisterTransactionValidatorRule() {
    // Arrange
    plugin.doRegister(serviceManager);

    // Act
    plugin.beforeExternalServices();

    // Assert
    verify(transactionValidatorService).registerTransactionValidatorRule(any());
  }

  @Test
  public void shouldRejectBlobTransactionsByDefault() {
    // Arrange
    when(lineaTransactionValidatorConfiguration.blobTxEnabled()).thenReturn(false);
    plugin.doRegister(serviceManager);
    plugin.beforeExternalServices();
    final TransactionValidationRule validatorRule = this.getTransactionValidatorRule();

    // Act - BLOB transaction
    when(transaction.getType()).thenReturn(TransactionType.BLOB);
    Optional<String> result = validatorRule.validate(transaction);

    // Assert
    assertThat(result).isPresent();
    assertThat(result.get())
        .isEqualTo(LineaTransactionValidatorError.BLOB_TX_NOT_ALLOWED.toString());
  }

  @Test
  public void shouldPermitEIP7702TransactionsByDefault() {
    // Arrange
    when(lineaTransactionValidatorConfiguration.delegateCodeTxEnabled()).thenReturn(true);
    plugin.doRegister(serviceManager);
    plugin.beforeExternalServices();
    final TransactionValidationRule validatorRule = this.getTransactionValidatorRule();

    // Act - EIP7702 transaction
    when(transaction.getType()).thenReturn(TransactionType.DELEGATE_CODE);
    Optional<String> result = validatorRule.validate(transaction);

    // Assert
    assertThat(result).isEmpty();
  }

  @Test
  public void shouldRejectEIP7702TransactionsWhenDisabled() {
    // Arrange
    when(lineaTransactionValidatorConfiguration.delegateCodeTxEnabled()).thenReturn(false);
    plugin.doRegister(serviceManager);
    plugin.beforeExternalServices();
    final TransactionValidationRule validatorRule = this.getTransactionValidatorRule();

    // Act - EIP7702 transaction
    when(transaction.getType()).thenReturn(TransactionType.DELEGATE_CODE);
    Optional<String> result = validatorRule.validate(transaction);

    // Assert
    assertThat(result).isPresent();
    assertThat(result.get())
        .isEqualTo(LineaTransactionValidatorError.DELEGATE_CODE_TX_NOT_ALLOWED.toString());
  }

  @Test
  public void shouldPermitLegacyTransactions() {
    // Arrange
    plugin.doRegister(serviceManager);
    plugin.beforeExternalServices();
    final TransactionValidationRule validatorRule = this.getTransactionValidatorRule();

    // Act - LEGACY/FRONTIER transaction
    when(transaction.getType()).thenReturn(TransactionType.FRONTIER);
    Optional<String> result = validatorRule.validate(transaction);

    // Assert
    assertThat(result).isEmpty();
  }

  @Test
  public void shouldPermitAccessListTransactions() {
    // Arrange
    plugin.doRegister(serviceManager);
    plugin.beforeExternalServices();
    final TransactionValidationRule validatorRule = this.getTransactionValidatorRule();

    // Act - ACCESS_LIST transaction
    when(transaction.getType()).thenReturn(TransactionType.ACCESS_LIST);
    Optional<String> result = validatorRule.validate(transaction);

    // Assert
    assertThat(result).isEmpty();
  }

  @Test
  public void shouldPermitEIP1559Transactions() {
    // Arrange
    plugin.doRegister(serviceManager);
    plugin.beforeExternalServices();
    final TransactionValidationRule validatorRule = this.getTransactionValidatorRule();

    // Act - EIP1559 transaction
    when(transaction.getType()).thenReturn(TransactionType.EIP1559);
    Optional<String> result = validatorRule.validate(transaction);

    // Assert
    assertThat(result).isEmpty();
  }

  @Test
  public void shouldPermitBlobTransactionsWhenEnabled() {
    // Arrange
    when(lineaTransactionValidatorConfiguration.blobTxEnabled()).thenReturn(true);
    plugin.doRegister(serviceManager);
    plugin.beforeExternalServices();
    final TransactionValidationRule validatorRule = this.getTransactionValidatorRule();

    // Act - BLOB transaction
    when(transaction.getType()).thenReturn(TransactionType.BLOB);
    Optional<String> result = validatorRule.validate(transaction);

    // Assert
    assertThat(result).isEmpty();
  }

  private TransactionValidationRule getTransactionValidatorRule() {
    ArgumentCaptor<TransactionValidationRule> ruleCaptor =
        ArgumentCaptor.forClass(TransactionValidationRule.class);
    verify(transactionValidatorService).registerTransactionValidatorRule(ruleCaptor.capture());
    return ruleCaptor.getValue();
  }
}
