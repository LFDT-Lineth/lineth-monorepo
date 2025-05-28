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

package net.consensys.linea.extradata;

import java.util.function.Consumer;
import java.util.function.Function;

import lombok.extern.slf4j.Slf4j;
import net.consensys.linea.config.LineaProfitabilityConfiguration;
import org.apache.commons.lang3.mutable.MutableLong;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.units.bigints.UInt32;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.plugin.services.RpcEndpointService;
import org.hyperledger.besu.plugin.services.rpc.RpcResponseType;

/**
 * Handles the Linea extra data custom extension.
 *
 * <p>In Linea the extra data field is used to distribute pricing config, it has a standard format,
 * it is versioned to support for changes in the format.
 *
 * <p>The version is the first byte of the extra data, currently on version 1 exists, in case the
 * version byte is not recognized as supported, then the extra data is simply ignored.
 */
@Slf4j
public class LineaExtraDataHandler {
  private final RpcEndpointService rpcEndpointService;
  private final ExtraDataConsumer[] extraDataConsumers;

  public LineaExtraDataHandler(
      final RpcEndpointService rpcEndpointService,
      final LineaProfitabilityConfiguration profitabilityConf) {
    this.rpcEndpointService = rpcEndpointService;
    this.extraDataConsumers = new ExtraDataConsumer[] {new Version1Consumer(profitabilityConf)};
  }

  /**
   * Handles the extra data, first tries to see if it has a supported format, if so the bytes are
   * processed according to that format.
   *
   * @param rawExtraData the extra data bytes
   * @throws LineaExtraDataException if the format of the extra data is invalid
   */
  public void handle(final Bytes rawExtraData) throws LineaExtraDataException {

    if (!Bytes.EMPTY.equals(rawExtraData)) {
      for (final ExtraDataConsumer extraDataConsumer : extraDataConsumers) {
        if (extraDataConsumer.canConsume(rawExtraData)) {
          // strip first byte since it is the version already used to select the actual consumer
          final var extraData = rawExtraData.slice(1);
          extraDataConsumer.accept(extraData);
          return;
        }
      }
      throw new LineaExtraDataException(
          LineaExtraDataException.ErrorType.INVALID_ARGUMENT,
          "Unsupported extra data field " + rawExtraData.toHexString());
    }
  }

  /** A consumer of a specific version of the extra data format */
  private interface ExtraDataConsumer extends Consumer<Bytes> {

    /**
     * Is this consumer able to process the given extra data?
     *
     * @param extraData extra data bytes
     * @return true if this consumer can process the extra data
     */
    boolean canConsume(Bytes extraData);

    static Long toLong(final Bytes fieldBytes) {
      return UInt32.fromBytes(fieldBytes).toLong();
    }
  }

  /**
   * Handles a version 1 extra data field and on successful parsing it updates the pricing config
   * and the min gas price
   *
   * <p>Version 1 has this format:
   *
   * <p>VERSION (1 byte) FIXED_COST (4 bytes) VARIABLE_COST (4 bytes) ETH_GAS_PRICE (4 bytes)
   */
  @SuppressWarnings("rawtypes")
  private class Version1Consumer implements ExtraDataConsumer {
    private static final int WEI_IN_KWEI = 1_000;
    private final LineaProfitabilityConfiguration profitabilityConf;
    private final FieldConsumer[] fieldsSequence;
    private final MutableLong currFixedCostKWei = new MutableLong();
    private final MutableLong currVariableCostKWei = new MutableLong();
    private final MutableLong currEthGasPriceKWei = new MutableLong();

    public Version1Consumer(final LineaProfitabilityConfiguration profitabilityConf) {
      this.profitabilityConf = profitabilityConf;

      final FieldConsumer fixedGasCostField =
          new FieldConsumer<>(
              "fixedGasCost", 4, ExtraDataConsumer::toLong, currFixedCostKWei::setValue);
      final FieldConsumer variableGasCostField =
          new FieldConsumer<>(
              "variableGasCost", 4, ExtraDataConsumer::toLong, currVariableCostKWei::setValue);
      final FieldConsumer ethGasPriceField =
          new FieldConsumer<>("ethGasPrice", 4, ExtraDataConsumer::toLong, this::updateEthGasPrice);

      this.fieldsSequence =
          new FieldConsumer[] {fixedGasCostField, variableGasCostField, ethGasPriceField};
    }

    public boolean canConsume(final Bytes rawExtraData) {
      return rawExtraData.get(0) == (byte) 1;
    }

    public synchronized void accept(final Bytes extraData) {
      log.debug("Parsing extra data version 1: {}", extraData.toHexString());
      int startIndex = 0;
      for (final FieldConsumer fieldConsumer : fieldsSequence) {
        fieldConsumer.accept(extraData.slice(startIndex, fieldConsumer.length));
        startIndex += fieldConsumer.length;
      }

      profitabilityConf.updateFixedVariableAndGasPrice(
          currFixedCostKWei.longValue() * WEI_IN_KWEI,
          currVariableCostKWei.longValue() * WEI_IN_KWEI,
          currEthGasPriceKWei.longValue() * WEI_IN_KWEI);
    }

    void updateEthGasPrice(final Long ethGasPriceKWei) {
      currEthGasPriceKWei.setValue(ethGasPriceKWei);
      if (profitabilityConf.extraDataSetMinGasPriceEnabled()) {
        final var minGasPriceWei = Wei.of(ethGasPriceKWei).multiply(WEI_IN_KWEI);
        final var resp =
            rpcEndpointService.call(
                "miner_setMinGasPrice", new Object[] {minGasPriceWei.toShortHexString()});
        if (!resp.getType().equals(RpcResponseType.SUCCESS)) {
          throw new LineaExtraDataException(
              LineaExtraDataException.ErrorType.FAILED_CALLING_SET_MIN_GAS_PRICE,
              "Internal setMinGasPrice method failed: " + resp);
        }
      } else {
        log.trace("Setting minGasPrice from extraData is disabled by conf");
      }
    }
  }

  private record FieldConsumer<T>(
      String name, int length, Function<Bytes, T> converter, Consumer<T> consumer)
      implements Consumer<Bytes> {

    @Override
    public void accept(final Bytes fieldBytes) {
      final var converted = converter.apply(fieldBytes);
      log.debug("Field {}={} (raw bytes: {})", name, converted, fieldBytes.toHexString());
      consumer.accept(converted);
    }
  }
}
