/*
 * Copyright Consensys Incorporated
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package lineth.rpc.methods;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

import java.util.Optional;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.StateOverrideMap;
import org.hyperledger.besu.ethereum.transaction.CallParameter;
import org.hyperledger.besu.ethereum.transaction.ImmutableCallParameter;
import org.hyperledger.besu.plugin.services.BesuConfiguration;
import org.hyperledger.besu.plugin.services.BlockchainService;
import org.hyperledger.besu.plugin.services.RpcEndpointService;
import org.hyperledger.besu.plugin.services.TransactionSimulationService;
import org.hyperledger.besu.plugin.services.rpc.PluginRpcResponse;
import org.hyperledger.besu.plugin.services.rpc.RpcResponseType;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;

class LineaEstimateGasTest {
  private static final Address SENDER =
      Address.fromHexString("0x1111111111111111111111111111111111111111");
  private static final Address RECIPIENT =
      Address.fromHexString("0x2222222222222222222222222222222222222222");

  private RpcEndpointService rpcEndpointService;
  private LineaEstimateGas lineaEstimateGas;

  @BeforeEach
  void setUp() {
    rpcEndpointService = mock(RpcEndpointService.class);
    final PluginRpcResponse response = mock(PluginRpcResponse.class);
    when(response.getType()).thenReturn(RpcResponseType.SUCCESS);
    when(response.getResult()).thenReturn("0x5208");
    when(rpcEndpointService.call(eq("eth_estimateGas"), any())).thenReturn(response);

    lineaEstimateGas =
        new LineaEstimateGas(
            mock(BesuConfiguration.class),
            mock(TransactionSimulationService.class),
            mock(BlockchainService.class),
            rpcEndpointService);
  }

  @Test
  void estimatesGasInStrictModeWhenStrictIsNotSet() {
    final CallParameter callParameter = callParameterBuilder().build();

    assertThat(lineaEstimateGas.getGasEstimation(callParameter, Optional.empty(), 0))
        .isEqualTo(21_000L);

    final Object[] params = capturedEstimateGasParams();
    assertThat(params).hasSize(2);
    assertThat(params[1]).isEqualTo("pending");
    final CallParameter sent = (CallParameter) params[0];
    assertThat(sent.getStrict()).contains(true);
    assertThat(sent.getSender()).contains(SENDER);
    assertThat(sent.getTo()).contains(RECIPIENT);
  }

  @Test
  void keepsStrictFalseSetByTheCaller() {
    final CallParameter callParameter = callParameterBuilder().strict(false).build();

    lineaEstimateGas.getGasEstimation(callParameter, Optional.empty(), 0);

    assertThat(((CallParameter) capturedEstimateGasParams()[0]).getStrict()).contains(false);
  }

  @Test
  void keepsStrictTrueSetByTheCaller() {
    final CallParameter callParameter = callParameterBuilder().strict(true).build();

    lineaEstimateGas.getGasEstimation(callParameter, Optional.empty(), 0);

    assertThat(((CallParameter) capturedEstimateGasParams()[0]).getStrict()).contains(true);
  }

  @Test
  void passesStateOverridesWithTheStrictCall() {
    final StateOverrideMap stateOverrides = new StateOverrideMap();

    lineaEstimateGas.getGasEstimation(
        callParameterBuilder().build(), Optional.of(stateOverrides), 0);

    final Object[] params = capturedEstimateGasParams();
    assertThat(params).hasSize(3);
    assertThat(((CallParameter) params[0]).getStrict()).contains(true);
    assertThat(params[1]).isEqualTo("pending");
    assertThat(params[2]).isSameAs(stateOverrides);
  }

  private static ImmutableCallParameter.Builder callParameterBuilder() {
    return ImmutableCallParameter.builder().sender(SENDER).to(RECIPIENT);
  }

  private Object[] capturedEstimateGasParams() {
    final ArgumentCaptor<Object[]> params = ArgumentCaptor.forClass(Object[].class);
    verify(rpcEndpointService).call(eq("eth_estimateGas"), params.capture());
    return params.getValue();
  }
}
