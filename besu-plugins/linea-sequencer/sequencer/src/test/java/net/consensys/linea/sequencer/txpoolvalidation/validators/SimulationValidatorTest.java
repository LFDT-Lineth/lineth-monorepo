/*
 * Copyright Consensys Software Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with
 * the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on
 * an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations under the License.
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package net.consensys.linea.sequencer.txpoolvalidation.validators;

import static com.github.tomakehurst.wiremock.client.WireMock.aResponse;
import static com.github.tomakehurst.wiremock.client.WireMock.equalTo;
import static com.github.tomakehurst.wiremock.client.WireMock.exactly;
import static com.github.tomakehurst.wiremock.client.WireMock.matchingJsonPath;
import static com.github.tomakehurst.wiremock.client.WireMock.post;
import static com.github.tomakehurst.wiremock.client.WireMock.postRequestedFor;
import static com.github.tomakehurst.wiremock.client.WireMock.stubFor;
import static com.github.tomakehurst.wiremock.client.WireMock.urlEqualTo;
import static com.github.tomakehurst.wiremock.client.WireMock.verify;
import static java.util.concurrent.TimeUnit.SECONDS;
import static org.assertj.core.api.Assertions.assertThat;
import static org.awaitility.Awaitility.await;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

import java.io.IOException;
import java.math.BigInteger;
import java.net.MalformedURLException;
import java.net.URI;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.HashMap;
import java.util.Map;
import java.util.Optional;

import com.github.tomakehurst.wiremock.junit5.WireMockRuntimeInfo;
import com.github.tomakehurst.wiremock.junit5.WireMockTest;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import net.consensys.linea.config.LineaNodeType;
import net.consensys.linea.config.LineaRejectedTxReportingConfiguration;
import net.consensys.linea.config.LineaTracerConfiguration;
import net.consensys.linea.config.LineaTransactionPoolValidatorConfiguration;
import net.consensys.linea.jsonrpc.JsonRpcManager;
import net.consensys.linea.plugins.config.LineaL1L2BridgeSharedConfiguration;
import net.consensys.linea.sequencer.modulelimit.ModuleLineCountValidator;
import net.consensys.linea.sequencer.txselection.selectors.TraceLineLimitTransactionSelectorTest;
import org.apache.tuweni.bytes.Bytes;
import org.bouncycastle.asn1.sec.SECNamedCurves;
import org.bouncycastle.asn1.x9.X9ECParameters;
import org.bouncycastle.crypto.params.ECDomainParameters;
import org.hyperledger.besu.crypto.SECPSignature;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.ethereum.core.BlockHeader;
import org.hyperledger.besu.plugin.services.BlockchainService;
import org.hyperledger.besu.plugin.services.TransactionSimulationService;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.junit.jupiter.api.io.TempDir;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;

@Slf4j
@RequiredArgsConstructor
@WireMockTest
@ExtendWith(MockitoExtension.class)
public class SimulationValidatorTest {
  private static final String MODULE_LINE_LIMITS_RESOURCE_NAME = "/sequencer/line-limits.toml";
  public static final Address SENDER =
      Address.fromHexString("0x0000000000000000000000000000000000001000");
  public static final Address RECIPIENT =
      Address.fromHexString("0x0000000000000000000000000000000000001001");
  private static final Wei BASE_FEE = Wei.of(7);
  private static final Wei PROFITABLE_GAS_PRICE = Wei.of(11000000);
  private static final SECPSignature FAKE_SIGNATURE;
  private static final Address BRIDGE_CONTRACT =
      Address.fromHexString("0x508Ca82Df566dCD1B0DE8296e70a96332cD644ec");
  private static final Bytes BRIDGE_LOG_TOPIC =
      Bytes.fromHexString("e856c2b8bd4eb0027ce32eeaf595c21b0b6b4644b326e5b7bd80a1cf8db72e6c");

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

  private Map<String, Integer> lineCountLimits;

  @Mock BlockchainService blockchainService;
  @Mock TransactionSimulationService transactionSimulationService;
  private JsonRpcManager jsonRpcManager;
  @TempDir private Path tempDataDir;
  @TempDir static Path tempDir;
  static Path lineLimitsConfPath;

  @BeforeAll
  public static void beforeAll() throws IOException {
    lineLimitsConfPath = tempDir.resolve("line-limits.toml");
    Files.copy(
        TraceLineLimitTransactionSelectorTest.class.getResourceAsStream(
            MODULE_LINE_LIMITS_RESOURCE_NAME),
        lineLimitsConfPath);
  }

  @BeforeEach
  public void initialize(final WireMockRuntimeInfo wmInfo) throws MalformedURLException {
    final var tracerConf =
        LineaTracerConfiguration.builder()
            .moduleLimitsFilePath(lineLimitsConfPath.toString())
            .build();
    lineCountLimits = new HashMap<>(ModuleLineCountValidator.createLimitModules(tracerConf));
    final var pendingBlockHeader = mock(BlockHeader.class);
    when(pendingBlockHeader.getBaseFee()).thenReturn(Optional.of(BASE_FEE));
    when(pendingBlockHeader.getCoinbase()).thenReturn(Address.ZERO);
    when(transactionSimulationService.simulatePendingBlockHeader()).thenReturn(pendingBlockHeader);
    when(blockchainService.getChainId()).thenReturn(Optional.of(BigInteger.ONE));

    final var rejectedTxReportingConf =
        LineaRejectedTxReportingConfiguration.builder()
            .rejectedTxEndpoint(URI.create(wmInfo.getHttpBaseUrl()).toURL())
            .lineaNodeType(LineaNodeType.P2P)
            .build();
    jsonRpcManager =
        new JsonRpcManager("simulation-test", tempDataDir, rejectedTxReportingConf).start();

    // rejected tx json-rpc stubbing
    stubFor(
        post(urlEqualTo("/"))
            .willReturn(
                aResponse()
                    .withStatus(200)
                    .withHeader("Content-Type", "application/json")
                    .withBody(
                        "{\"jsonrpc\":\"2.0\",\"result\":{ \"status\": \"SAVED\"},\"id\":1}")));
  }

  @AfterEach
  void cleanup() {
    jsonRpcManager.shutdown();
  }

  private SimulationValidator createSimulationValidator(
      final Map<String, Integer> lineCountLimits,
      final boolean enableForApi,
      final boolean enableForP2p) {
    return new SimulationValidator(
        blockchainService,
        transactionSimulationService,
        LineaTransactionPoolValidatorConfiguration.builder()
            .txPoolSimulationCheckApiEnabled(enableForApi)
            .txPoolSimulationCheckP2pEnabled(enableForP2p)
            .build(),
        lineCountLimits,
        LineaL1L2BridgeSharedConfiguration.builder()
            .contract(BRIDGE_CONTRACT)
            .topic(BRIDGE_LOG_TOPIC)
            .build(),
        Optional.of(jsonRpcManager));
  }

  @Test
  public void successfulTransactionIsValid() {
    final var simulationValidator = createSimulationValidator(lineCountLimits, true, false);
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
    assertThat(simulationValidator.validateTransaction(transaction, true, false)).isEmpty();
  }

  @Test
  public void moduleLineCountOverflowTransactionIsInvalidAndReported() {
    lineCountLimits.put("EXT", 5);
    final var simulationValidator = createSimulationValidator(lineCountLimits, true, false);
    final org.hyperledger.besu.ethereum.core.Transaction transaction =
        org.hyperledger.besu.ethereum.core.Transaction.builder()
            .sender(SENDER)
            .to(RECIPIENT)
            .gasLimit(21000)
            .gasPrice(PROFITABLE_GAS_PRICE)
            .payload(Bytes.repeat((byte) 1, 1000))
            .value(Wei.ONE)
            .signature(FAKE_SIGNATURE)
            .build();
    final var expectedReasonMessage =
        "Transaction 0xbf668c5dc926c008d5b34f347e1842b94911b46f4a36b668812f821e20303322 line count for module EXT=7 is above the limit 5";
    assertThat(simulationValidator.validateTransaction(transaction, true, false))
        .contains(expectedReasonMessage);

    // assert that wiremock received 1 post request for rejected tx.
    // Use Awaitility to wait for the condition to be met
    await()
        .atMost(6, SECONDS)
        .untilAsserted(
            () ->
                verify(
                    exactly(1),
                    postRequestedFor(urlEqualTo("/"))
                        .withRequestBody(
                            matchingJsonPath(
                                "$.params.txRejectionStage", equalTo(LineaNodeType.P2P.name())))
                        .withRequestBody(
                            matchingJsonPath(
                                "$.params.reasonMessage", equalTo(expectedReasonMessage)))));
  }
}
