/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package net.consensys.linea.sequencer.liveness;

import static org.assertj.core.api.Assertions.*;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.Mockito.*;

import java.io.IOException;
import java.nio.file.Path;
import java.time.Duration;
import java.time.Instant;
import java.util.Optional;
import net.consensys.linea.bundles.TransactionBundle;
import net.consensys.linea.config.LineaLivenessServiceConfiguration;
import net.consensys.linea.metrics.LineaMetricCategory;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.hyperledger.besu.plugin.services.*;
import org.hyperledger.besu.plugin.services.metrics.Counter;
import org.hyperledger.besu.plugin.services.metrics.LabelledSuppliedMetric;
import org.hyperledger.besu.plugin.services.metrics.MetricCategoryRegistry;
import org.hyperledger.besu.plugin.services.rpc.PluginRpcResponse;
import org.hyperledger.besu.plugin.services.rpc.RpcResponseType;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.junit.jupiter.api.io.TempDir;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.mockito.junit.jupiter.MockitoSettings;
import org.mockito.quality.Strictness;

@ExtendWith(MockitoExtension.class)
@MockitoSettings(strictness = Strictness.LENIENT)
public class LineaLivenessServiceTest {

  @TempDir private Path tempDir;

  @Mock private MetricCategoryRegistry metricCategoryRegistry;
  @Mock private MetricsSystem metricsSystem;
  @Mock private RpcEndpointService rpcEndpointService;
  @Mock private LivenessTxBuilder livenessTxBuilder;
  @Mock private Counter counter;
  @Mock private LabelledSuppliedMetric labelledSuppliedMetric;
  @Mock private LineaLivenessServiceConfiguration lineaLivenessServiceConfiguration;
  @Mock private Transaction transaction;

  private LivenessService livenessService;
  private long nonce = 100L;

  private static final String CONTRACT_ADDRESS = "0x1234567890123456789012345678901234567890";
  private static final String SIGNER_URL = "http://localhost:9000";
  private static final String SIGNER_KEY_ID = "test-key";
  private static final String SIGNER_ADDRESS = "0x1234567890123456789012345678901234567890";
  private static final long MAX_BLOCK_AGE_SECONDS = 10;
  private static final long BUNDLE_MAX_TIMESTAMP_SURPLUS_SECONDS = 12;
  private static final long GAS_LIMIT = 100000;
  private static final long GAS_PRICE = 1000000000; // 1Gwei

  @BeforeEach
  public void setUp() throws IOException {
    // Mock metrics
    when(metricsSystem.createCounter(any(), anyString(), anyString())).thenReturn(counter);
    when(metricsSystem.createLabelledSuppliedGauge(any(), anyString(), anyString(), anyString()))
        .thenReturn(labelledSuppliedMetric);

    // Mock metric category registry
    when(metricCategoryRegistry.isMetricCategoryEnabled(eq(LineaMetricCategory.SEQUENCER_LIVENESS)))
        .thenReturn(true);

    // Mock rpc endpoint service
    when(rpcEndpointService.call(eq("eth_getTransactionCount"), any()))
        .thenReturn(
            new PluginRpcResponse() {
              @Override
              public Object getResult() {
                return "0x" + Long.toHexString(nonce += 2);
              }

              @Override
              public RpcResponseType getType() {
                return RpcResponseType.SUCCESS;
              }
            });

    // Mock liveness tx builder
    when(livenessTxBuilder.buildUptimeTransaction(anyBoolean(), anyLong(), anyLong()))
        .thenReturn(transaction);

    // Mock the configurations
    setupDefaultConfiguration();
  }

  private void setupDefaultConfiguration() {
    when(lineaLivenessServiceConfiguration.enabled()).thenReturn(true);
    when(lineaLivenessServiceConfiguration.contractAddress()).thenReturn(CONTRACT_ADDRESS);
    when(lineaLivenessServiceConfiguration.signerUrl()).thenReturn(SIGNER_URL);
    when(lineaLivenessServiceConfiguration.signerKeyId()).thenReturn(SIGNER_KEY_ID);
    when(lineaLivenessServiceConfiguration.signerAddress()).thenReturn(SIGNER_ADDRESS);
    when(lineaLivenessServiceConfiguration.maxBlockAgeSeconds())
        .thenReturn(Duration.ofSeconds(MAX_BLOCK_AGE_SECONDS));
    when(lineaLivenessServiceConfiguration.bundleMaxTimestampSurplusSecond())
        .thenReturn(Duration.ofSeconds(BUNDLE_MAX_TIMESTAMP_SURPLUS_SECONDS));
    when(lineaLivenessServiceConfiguration.gasLimit()).thenReturn(GAS_LIMIT);
    when(lineaLivenessServiceConfiguration.gasPrice()).thenReturn(GAS_PRICE);
  }

  @Test
  public void shouldReturnEmptyIfLivenessIsDisabled() {
    when(lineaLivenessServiceConfiguration.enabled()).thenReturn(false);

    livenessService =
        new LineaLivenessService(
            lineaLivenessServiceConfiguration,
            rpcEndpointService,
            livenessTxBuilder,
            metricCategoryRegistry,
            metricsSystem);

    long currentTime = Instant.now().getEpochSecond();
    Optional<TransactionBundle> bundle =
        livenessService.checkBlockTimestampAndBuildBundle(currentTime, 10000000, 10);

    assertThat(bundle.isPresent()).isFalse();
  }

  @Test
  public void shouldNotCallMetricFunctionsIfMetricIsDisabled() {
    when(lineaLivenessServiceConfiguration.enabled()).thenReturn(true);
    when(metricCategoryRegistry.isMetricCategoryEnabled(eq(LineaMetricCategory.SEQUENCER_LIVENESS)))
        .thenReturn(false);

    livenessService =
        new LineaLivenessService(
            lineaLivenessServiceConfiguration,
            rpcEndpointService,
            livenessTxBuilder,
            metricCategoryRegistry,
            metricsSystem);

    verify(metricsSystem, never()).createCounter(any(), any(), any());
  }

  @Test
  public void shouldReturnEmptyBundleIfLastBlockTimestampHasBeenChecked() {
    livenessService =
        new LineaLivenessService(
            lineaLivenessServiceConfiguration,
            rpcEndpointService,
            livenessTxBuilder,
            metricCategoryRegistry,
            metricsSystem);

    long currentTime = Instant.now().getEpochSecond();
    long lastBlockTimestamp = currentTime - 11; // 10 sec is the max block timestamp lag allowed
    long targetBlockNumber = 10;

    livenessService.checkBlockTimestampAndBuildBundle(
        currentTime, lastBlockTimestamp, targetBlockNumber);

    // simulate when the same last block had been checked
    Optional<TransactionBundle> bundle =
        livenessService.checkBlockTimestampAndBuildBundle(
            currentTime + 10, lastBlockTimestamp, targetBlockNumber);

    assertThat(bundle.isPresent()).isFalse();
  }

  @Test
  public void shouldReturnEmptyBundleIfTargetBlockNumberIsOne() {
    livenessService =
        new LineaLivenessService(
            lineaLivenessServiceConfiguration,
            rpcEndpointService,
            livenessTxBuilder,
            metricCategoryRegistry,
            metricsSystem);

    long currentTime = Instant.now().getEpochSecond();
    long lastBlockTimestamp = currentTime - 11; // 10 sec is the max block timestamp lag allowed
    long targetBlockNumber = 1;

    // simulate when targetBlockNumber is 1 and last block is the genesis block
    Optional<TransactionBundle> bundle =
        livenessService.checkBlockTimestampAndBuildBundle(
            currentTime, lastBlockTimestamp, targetBlockNumber);

    // should not return any liveness bundle
    assertThat(bundle.isPresent()).isFalse();
  }

  @Test
  public void shouldReturnValidBundleIfFirstBlockIsLate() throws IOException {
    livenessService =
        new LineaLivenessService(
            lineaLivenessServiceConfiguration,
            rpcEndpointService,
            livenessTxBuilder,
            metricCategoryRegistry,
            metricsSystem);

    // simulate the first checked block was late
    long currentTime = Instant.now().getEpochSecond();
    long lastBlockTimestamp = currentTime - 11; // 10 sec is the max block timestamp lag allowed
    long targetBlockNumber = 10;

    Optional<TransactionBundle> bundle =
        livenessService.checkBlockTimestampAndBuildBundle(
            currentTime, lastBlockTimestamp, targetBlockNumber);

    assertThat(bundle.isPresent()).isTrue();

    // should report the lastBlockTimestamp as down blocktime
    verify(livenessTxBuilder, times(1))
        .buildUptimeTransaction(eq(false), eq(lastBlockTimestamp), eq(102L));
    verify(livenessTxBuilder, times(1)).buildUptimeTransaction(eq(true), eq(currentTime), eq(103L));
  }

  @Test
  public void shouldReturnValidBundleWhenSecondBlockArrivedLate() throws Exception {
    livenessService =
        new LineaLivenessService(
            lineaLivenessServiceConfiguration,
            rpcEndpointService,
            livenessTxBuilder,
            metricCategoryRegistry,
            metricsSystem);

    // simulate the first checked block elapsed time is within 10 sec
    long currentTime = Instant.now().getEpochSecond();
    long firstLastBlockTimestamp = currentTime - 1;
    long targetBlockNumber = 10;

    Optional<TransactionBundle> bundle =
        livenessService.checkBlockTimestampAndBuildBundle(
            currentTime, firstLastBlockTimestamp, targetBlockNumber);

    // should not return any liveness bundle
    assertThat(bundle.isPresent()).isFalse();

    // simulate the second block arrived late
    currentTime = currentTime + 10;
    long secondLastBlockTimestamp =
        currentTime - 1; // the provided lastBlockTimestamp is always very close to the current time
    targetBlockNumber = 11;

    bundle =
        livenessService.checkBlockTimestampAndBuildBundle(
            currentTime, secondLastBlockTimestamp, targetBlockNumber);

    assertThat(bundle.isPresent()).isTrue();

    // should report the firstLastBlockTimestamp as down blocktime
    verify(livenessTxBuilder, times(1))
        .buildUptimeTransaction(eq(false), eq(firstLastBlockTimestamp), eq(102L));
    verify(livenessTxBuilder, times(1)).buildUptimeTransaction(eq(true), eq(currentTime), eq(103L));
  }

  @Test
  public void shouldReturnValidBundleWhenFirstLateBlockWasNotReported() throws Exception {
    livenessService =
        new LineaLivenessService(
            lineaLivenessServiceConfiguration,
            rpcEndpointService,
            livenessTxBuilder,
            metricCategoryRegistry,
            metricsSystem);

    // simulate the first checked block elapsed time is within 10 sec
    long currentTime = Instant.now().getEpochSecond();
    long firstLastBlockTimestamp = currentTime - 1;
    long targetBlockNumber = 10;

    Optional<TransactionBundle> bundle =
        livenessService.checkBlockTimestampAndBuildBundle(
            currentTime, firstLastBlockTimestamp, targetBlockNumber);

    // should not return any liveness bundle
    assertThat(bundle.isPresent()).isFalse();

    // simulate the second block arrived late
    currentTime = currentTime + 10;
    long secondLastBlockTimestamp =
        currentTime - 1; // the provided lastBlockTimestamp is always very close to the current time
    targetBlockNumber = 11;

    bundle =
        livenessService.checkBlockTimestampAndBuildBundle(
            currentTime, secondLastBlockTimestamp, targetBlockNumber);

    assertThat(bundle.isPresent()).isTrue();

    // should report the firstLastBlockTimestamp as down blocktime
    verify(livenessTxBuilder, times(1))
        .buildUptimeTransaction(eq(false), eq(firstLastBlockTimestamp), eq(102L));
    verify(livenessTxBuilder, times(1)).buildUptimeTransaction(eq(true), eq(currentTime), eq(103L));

    // notify that the first late block was failed to report
    livenessService.updateUptimeMetrics(false, firstLastBlockTimestamp);

    // simulate the third block arrived on time
    currentTime = currentTime + 2;
    long thirdLastBlockTimestamp = currentTime - 1;
    targetBlockNumber = 12;

    bundle =
        livenessService.checkBlockTimestampAndBuildBundle(
            currentTime, thirdLastBlockTimestamp, targetBlockNumber);

    assertThat(bundle.isPresent()).isTrue();

    // should still report the firstLastBlockTimestamp as down blocktime
    verify(livenessTxBuilder, times(1))
        .buildUptimeTransaction(eq(false), eq(firstLastBlockTimestamp), eq(104L));
    verify(livenessTxBuilder, times(1)).buildUptimeTransaction(eq(true), eq(currentTime), eq(105L));
  }

  @Test
  public void shouldReturnValidBundleWhenMultipleLateBlocks() throws Exception {
    livenessService =
        new LineaLivenessService(
            lineaLivenessServiceConfiguration,
            rpcEndpointService,
            livenessTxBuilder,
            metricCategoryRegistry,
            metricsSystem);

    // simulate the first checked block arrived on time
    long currentTime = Instant.now().getEpochSecond();
    long firstLastBlockTimestamp = currentTime - 1;
    long targetBlockNumber = 10;

    Optional<TransactionBundle> bundle =
        livenessService.checkBlockTimestampAndBuildBundle(
            currentTime, firstLastBlockTimestamp, targetBlockNumber);

    // no liveness bundle should send
    assertThat(bundle.isPresent()).isFalse();

    // simulate the second block arrived late
    currentTime = currentTime + 10;
    long secondLastBlockTimestamp = currentTime - 1;
    targetBlockNumber = 11;

    bundle =
        livenessService.checkBlockTimestampAndBuildBundle(
            currentTime, secondLastBlockTimestamp, targetBlockNumber);
    assertThat(bundle.isPresent()).isTrue();

    // should report the firstLastBlockTimestamp as down blocktime
    verify(livenessTxBuilder, times(1))
        .buildUptimeTransaction(eq(false), eq(firstLastBlockTimestamp), eq(102L));
    verify(livenessTxBuilder, times(1)).buildUptimeTransaction(eq(true), eq(currentTime), eq(103L));

    // notify that the first late block was succeeded to report
    livenessService.updateUptimeMetrics(true, secondLastBlockTimestamp);

    // simulate the third block arrived late
    currentTime = currentTime + 10;
    long thirdLastBlockTimestamp = currentTime - 1;
    targetBlockNumber = 12;

    bundle =
        livenessService.checkBlockTimestampAndBuildBundle(
            currentTime, thirdLastBlockTimestamp, targetBlockNumber);
    assertThat(bundle.isPresent()).isTrue();

    // should report the secondLastBlockTimestamp as down blocktime
    verify(livenessTxBuilder, times(1))
        .buildUptimeTransaction(eq(false), eq(secondLastBlockTimestamp), eq(104L));
    verify(livenessTxBuilder, times(1)).buildUptimeTransaction(eq(true), eq(currentTime), eq(105L));

    // notify that the second late block was succeeded to report
    livenessService.updateUptimeMetrics(true, thirdLastBlockTimestamp);

    // simulate the fourth block arrived late
    currentTime = currentTime + 10;
    long fourthLastBlockTimestamp = currentTime - 1;
    targetBlockNumber = 13;

    bundle =
        livenessService.checkBlockTimestampAndBuildBundle(
            currentTime, fourthLastBlockTimestamp, targetBlockNumber);
    assertThat(bundle.isPresent()).isTrue();

    // should report the thirdLastBlockTimestamp as down blocktime
    verify(livenessTxBuilder, times(1))
        .buildUptimeTransaction(eq(false), eq(thirdLastBlockTimestamp), eq(106L));
    verify(livenessTxBuilder, times(1)).buildUptimeTransaction(eq(true), eq(currentTime), eq(107L));

    // notify that the third late block was succeeded to report
    livenessService.updateUptimeMetrics(true, fourthLastBlockTimestamp);
  }

  @Test
  public void shouldReturnValidBundleWhenMultipleLateBlocksNotReported() throws Exception {
    livenessService =
        new LineaLivenessService(
            lineaLivenessServiceConfiguration,
            rpcEndpointService,
            livenessTxBuilder,
            metricCategoryRegistry,
            metricsSystem);

    // simulate the first checked block arrived on time
    long currentTime = Instant.now().getEpochSecond();
    long firstLastBlockTimestamp = currentTime - 1;
    long targetBlockNumber = 10;

    Optional<TransactionBundle> bundle =
        livenessService.checkBlockTimestampAndBuildBundle(
            currentTime, firstLastBlockTimestamp, targetBlockNumber);

    // no liveness bundle should send
    assertThat(bundle.isPresent()).isFalse();

    // simulate the second block arrived late
    currentTime = currentTime + 10;
    long secondLastBlockTimestamp = currentTime - 1;
    targetBlockNumber = 11;

    bundle =
        livenessService.checkBlockTimestampAndBuildBundle(
            currentTime, secondLastBlockTimestamp, targetBlockNumber);

    assertThat(bundle.isPresent()).isTrue();

    // should report the firstLastBlockTimestamp as down blocktime
    verify(livenessTxBuilder, times(1))
        .buildUptimeTransaction(eq(false), eq(firstLastBlockTimestamp), eq(102L));
    verify(livenessTxBuilder, times(1)).buildUptimeTransaction(eq(true), eq(currentTime), eq(103L));

    // notify that the first late block was failed to report
    livenessService.updateUptimeMetrics(false, secondLastBlockTimestamp);

    // simulate the third block arrived late
    currentTime = currentTime + 10;
    long thirdLastBlockTimestamp = currentTime - 1;
    targetBlockNumber = 12;

    bundle =
        livenessService.checkBlockTimestampAndBuildBundle(
            currentTime, thirdLastBlockTimestamp, targetBlockNumber);

    assertThat(bundle.isPresent()).isTrue();

    // should still report the firstLastBlockTimestamp as down blocktime
    verify(livenessTxBuilder, times(1))
        .buildUptimeTransaction(eq(false), eq(firstLastBlockTimestamp), eq(104L));
    verify(livenessTxBuilder, times(1)).buildUptimeTransaction(eq(true), eq(currentTime), eq(105L));

    // notify that the first late block was failed to report
    livenessService.updateUptimeMetrics(false, thirdLastBlockTimestamp);

    // simulate the fourth block on time
    currentTime = currentTime + 2;
    long fourthLastBlockTimestamp = currentTime - 1;
    targetBlockNumber = 13;

    bundle =
        livenessService.checkBlockTimestampAndBuildBundle(
            currentTime, fourthLastBlockTimestamp, targetBlockNumber);

    assertThat(bundle.isPresent()).isTrue();

    // should still report the firstLastBlockTimestamp as down blocktime
    verify(livenessTxBuilder, times(1))
        .buildUptimeTransaction(eq(false), eq(firstLastBlockTimestamp), eq(106L));
    verify(livenessTxBuilder, times(1)).buildUptimeTransaction(eq(true), eq(currentTime), eq(107L));

    // notify that the first late block was finally succeeded to report
    livenessService.updateUptimeMetrics(true, fourthLastBlockTimestamp);

    // simulate the fifth block on time
    currentTime = currentTime + 2;
    long fifthLastBlockTimestamp = currentTime - 1;
    targetBlockNumber = 14;

    bundle =
        livenessService.checkBlockTimestampAndBuildBundle(
            currentTime, fifthLastBlockTimestamp, targetBlockNumber);

    // no liveness bundle should send
    assertThat(bundle.isPresent()).isFalse();

    // simulate the sixth block arrived late
    currentTime = currentTime + 10;
    long sixthLastBlockTimestamp = currentTime - 1;
    targetBlockNumber = 15;

    bundle =
        livenessService.checkBlockTimestampAndBuildBundle(
            currentTime, sixthLastBlockTimestamp, targetBlockNumber);

    assertThat(bundle.isPresent()).isTrue();

    // should report the fifthLastBlockTimestamp as down blocktime
    verify(livenessTxBuilder, times(1))
        .buildUptimeTransaction(eq(false), eq(fifthLastBlockTimestamp), eq(108L));
    verify(livenessTxBuilder, times(1)).buildUptimeTransaction(eq(true), eq(currentTime), eq(109L));
  }
}
