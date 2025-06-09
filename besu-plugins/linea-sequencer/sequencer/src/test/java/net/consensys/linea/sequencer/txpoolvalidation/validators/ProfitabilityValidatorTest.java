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

import java.math.BigInteger;
import java.util.Optional;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import net.consensys.linea.config.LineaProfitabilityCliOptions;
import org.apache.tuweni.bytes.Bytes;
import org.bouncycastle.asn1.sec.SECNamedCurves;
import org.bouncycastle.asn1.x9.X9ECParameters;
import org.bouncycastle.crypto.params.ECDomainParameters;
import org.hyperledger.besu.crypto.SECPSignature;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.plugin.services.BesuConfiguration;
import org.hyperledger.besu.plugin.services.BlockchainService;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;

@Slf4j
@RequiredArgsConstructor
@ExtendWith(MockitoExtension.class)
public class ProfitabilityValidatorTest {
  public static final Address SENDER =
      Address.fromHexString("0x0000000000000000000000000000000000001000");
  public static final Address RECIPIENT =
      Address.fromHexString("0x0000000000000000000000000000000000001001");
  private static Wei PROFITABLE_GAS_PRICE = Wei.of(11_000_000);
  private static Wei UNPROFITABLE_GAS_PRICE = Wei.of(200_000);
  private static final SECPSignature FAKE_SIGNATURE;

  static {
    final X9ECParameters params = SECNamedCurves.getByName("secp256k1");
    final ECDomainParameters curve =
        new ECDomainParameters(params.getCurve(), params.getG(), params.getN(), params.getH());
    FAKE_SIGNATURE =
        SECPSignature.create(
            new BigInteger(
                "66397251408932042429874251838229702988618145381408295790259650671563847073199"),
            new BigInteger(
                "24729624138373455972486746091821238755870276413282629437244319694880507882088"),
            (byte) 0,
            curve.getN());
  }

  public static final double TX_POOL_MIN_MARGIN = 0.5;
  private ProfitabilityValidator profitabilityValidatorAlways;
  private ProfitabilityValidator profitabilityValidatorOnlyApi;
  private ProfitabilityValidator profitabilityValidatorOnlyP2p;
  private ProfitabilityValidator profitabilityValidatorNever;

  @Mock BesuConfiguration besuConfiguration;
  @Mock BlockchainService blockchainService;

  @BeforeEach
  public void initialize() {
    final var profitabilityConfBuilder =
        LineaProfitabilityCliOptions.create().toDomainObject().toBuilder()
            .txPoolMinMargin(TX_POOL_MIN_MARGIN);

    profitabilityValidatorAlways =
        new ProfitabilityValidator(
            besuConfiguration,
            blockchainService,
            profitabilityConfBuilder
                .txPoolCheckP2pEnabled(true)
                .txPoolCheckApiEnabled(true)
                .build());

    profitabilityValidatorNever =
        new ProfitabilityValidator(
            besuConfiguration,
            blockchainService,
            profitabilityConfBuilder
                .txPoolCheckP2pEnabled(false)
                .txPoolCheckApiEnabled(false)
                .build());

    profitabilityValidatorOnlyApi =
        new ProfitabilityValidator(
            besuConfiguration,
            blockchainService,
            profitabilityConfBuilder
                .txPoolCheckP2pEnabled(false)
                .txPoolCheckApiEnabled(true)
                .build());

    profitabilityValidatorOnlyP2p =
        new ProfitabilityValidator(
            besuConfiguration,
            blockchainService,
            profitabilityConfBuilder
                .txPoolCheckP2pEnabled(true)
                .txPoolCheckApiEnabled(false)
                .build());
  }

  @Test
  public void acceptPriorityRemoteWhenBelowMinProfitability() {
    final org.hyperledger.besu.ethereum.core.Transaction transaction =
        org.hyperledger.besu.ethereum.core.Transaction.builder()
            .sender(SENDER)
            .to(RECIPIENT)
            .gasLimit(21000)
            .gasPrice(PROFITABLE_GAS_PRICE)
            .payload(Bytes.EMPTY)
            .value(Wei.ONE)
            .signature(FAKE_SIGNATURE)
            .build();
    assertThat(profitabilityValidatorAlways.validateTransaction(transaction, false, true))
        .isEmpty();
  }

  @Test
  public void rejectRemoteWhenBelowMinProfitability() {
    when(besuConfiguration.getMinGasPrice()).thenReturn(Wei.of(100_000_000));
    when(blockchainService.getNextBlockBaseFee()).thenReturn(Optional.of(Wei.of(7)));
    final org.hyperledger.besu.ethereum.core.Transaction transaction =
        org.hyperledger.besu.ethereum.core.Transaction.builder()
            .sender(SENDER)
            .to(RECIPIENT)
            .gasLimit(21000)
            .gasPrice(UNPROFITABLE_GAS_PRICE)
            .payload(Bytes.EMPTY)
            .value(Wei.ONE)
            .signature(FAKE_SIGNATURE)
            .build();
    assertThat(profitabilityValidatorAlways.validateTransaction(transaction, false, false))
        .isPresent()
        .contains("Gas price too low");
  }

  @Test
  public void acceptRemoteWhenBelowMinProfitabilityIfCheckNeverEnabled() {
    final org.hyperledger.besu.ethereum.core.Transaction transaction =
        org.hyperledger.besu.ethereum.core.Transaction.builder()
            .sender(SENDER)
            .to(RECIPIENT)
            .gasLimit(21000)
            .gasPrice(UNPROFITABLE_GAS_PRICE)
            .payload(Bytes.EMPTY)
            .value(Wei.ONE)
            .signature(FAKE_SIGNATURE)
            .build();
    assertThat(profitabilityValidatorNever.validateTransaction(transaction, false, false))
        .isEmpty();
  }

  @Test
  public void acceptLocalWhenBelowMinProfitabilityIfCheckNeverEnabled() {
    final org.hyperledger.besu.ethereum.core.Transaction transaction =
        org.hyperledger.besu.ethereum.core.Transaction.builder()
            .sender(SENDER)
            .to(RECIPIENT)
            .gasLimit(21000)
            .gasPrice(UNPROFITABLE_GAS_PRICE)
            .payload(Bytes.EMPTY)
            .value(Wei.ONE)
            .signature(FAKE_SIGNATURE)
            .build();
    assertThat(profitabilityValidatorNever.validateTransaction(transaction, true, false)).isEmpty();
  }

  @Test
  public void acceptRemoteWhenBelowMinProfitabilityIfCheckDisabledForP2p() {
    final org.hyperledger.besu.ethereum.core.Transaction transaction =
        org.hyperledger.besu.ethereum.core.Transaction.builder()
            .sender(SENDER)
            .to(RECIPIENT)
            .gasLimit(21000)
            .gasPrice(UNPROFITABLE_GAS_PRICE)
            .payload(Bytes.EMPTY)
            .value(Wei.ONE)
            .signature(FAKE_SIGNATURE)
            .build();
    assertThat(profitabilityValidatorOnlyApi.validateTransaction(transaction, false, false))
        .isEmpty();
  }

  @Test
  public void rejectRemoteWhenBelowMinProfitabilityIfCheckEnableForP2p() {
    when(besuConfiguration.getMinGasPrice()).thenReturn(Wei.of(100_000_000));
    when(blockchainService.getNextBlockBaseFee()).thenReturn(Optional.of(Wei.of(7)));
    final org.hyperledger.besu.ethereum.core.Transaction transaction =
        org.hyperledger.besu.ethereum.core.Transaction.builder()
            .sender(SENDER)
            .to(RECIPIENT)
            .gasLimit(21000)
            .gasPrice(UNPROFITABLE_GAS_PRICE)
            .payload(Bytes.EMPTY)
            .value(Wei.ONE)
            .signature(FAKE_SIGNATURE)
            .build();
    assertThat(profitabilityValidatorOnlyP2p.validateTransaction(transaction, false, false))
        .isPresent()
        .contains("Gas price too low");
  }

  @Test
  public void acceptLocalWhenBelowMinProfitabilityIfCheckDisabledForApi() {
    final org.hyperledger.besu.ethereum.core.Transaction transaction =
        org.hyperledger.besu.ethereum.core.Transaction.builder()
            .sender(SENDER)
            .to(RECIPIENT)
            .gasLimit(21000)
            .gasPrice(UNPROFITABLE_GAS_PRICE)
            .payload(Bytes.EMPTY)
            .value(Wei.ONE)
            .signature(FAKE_SIGNATURE)
            .build();
    assertThat(profitabilityValidatorOnlyP2p.validateTransaction(transaction, true, false))
        .isEmpty();
  }

  @Test
  public void rejectLocalWhenBelowMinProfitabilityIfCheckEnableForApi() {
    when(besuConfiguration.getMinGasPrice()).thenReturn(Wei.of(100_000_000));
    when(blockchainService.getNextBlockBaseFee()).thenReturn(Optional.of(Wei.of(7)));
    final org.hyperledger.besu.ethereum.core.Transaction transaction =
        org.hyperledger.besu.ethereum.core.Transaction.builder()
            .sender(SENDER)
            .to(RECIPIENT)
            .gasLimit(21000)
            .gasPrice(UNPROFITABLE_GAS_PRICE)
            .payload(Bytes.EMPTY)
            .value(Wei.ONE)
            .signature(FAKE_SIGNATURE)
            .build();
    assertThat(profitabilityValidatorOnlyApi.validateTransaction(transaction, true, false))
        .isPresent()
        .contains("Gas price too low");
  }
}
