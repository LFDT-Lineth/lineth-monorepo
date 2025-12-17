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

package net.consensys.linea.zktracer.opcode;

import static net.consensys.linea.zktracer.Fork.toCamelCase;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.module.SimpleModule;
import com.fasterxml.jackson.databind.type.CollectionType;
import com.fasterxml.jackson.databind.type.TypeFactory;
import java.io.IOException;
import java.util.Arrays;
import java.util.List;
import java.util.Optional;
import java.util.concurrent.ConcurrentHashMap;
import lombok.SneakyThrows;
import lombok.extern.slf4j.Slf4j;
import net.consensys.linea.zktracer.Fork;
import net.consensys.linea.zktracer.json.JsonConverter;
import net.consensys.linea.zktracer.opcode.gas.Billing;
import net.consensys.linea.zktracer.opcode.gas.BillingDeserializer;

@Slf4j
/**
 * Responsible for managing opcode loading and opcode metadata retrieval. The design here is not
 * ideal for a concurrent setting, but it works. Essentially, the assumption is that loadOpcodes is
 * called before any attempt is made to access the opCodeDataList directly.
 */
public class OpCodes {
  private static final short OPCODES_LIST_SIZE = 256;

  /**
   * Map of fork to loaded opcode information. The purpose of this map is to prevent a given YAML
   * file from being loaded more than once. In actual fact, this could happen if we have concurrent
   * accesses which are very close together --- but that is not a real concern.
   */
  private static final ConcurrentHashMap<Fork, OpCodes> opCodesMap = new ConcurrentHashMap<>();

  /**
   * Loads all opcode metadata for a given fork. This may result in loading the necessary
   * information from a corresponding yaml file. Howwever, this method caches OpCodes instances to
   * prevent this happening more than once.
   *
   * @param fork Fork for which opcode metadata is required.
   * @return
   */
  public static OpCodes load(Fork fork) {
    OpCodes instance = opCodesMap.get(fork);
    //
    if (instance == null) {
      instance = new OpCodes(fork);
      opCodesMap.put(fork, instance);
    }
    // Done
    return instance;
  }

  /** Opcode data appropriate for a given fork. */
  private final OpCodeData[] opcodes = new OpCodeData[OPCODES_LIST_SIZE];

  /**
   * Construct a new instance of OpCodes for a given fork. This necessarily loads the opcode
   * information from the corresponding YAML file.
   *
   * @param fork
   */
  private OpCodes(Fork fork) {
    for (OpCodeData opCodeData : opCodesLocal(fork)) {
      opcodes[opCodeData.value()] = opCodeData;
    }
  }

  /**
   * isValid checks whether or not a given opcode index corresponds with a real opcode.
   *
   * @param value
   * @return
   */
  public boolean isValid(final int value) {
    if (value < 0 || value > 255) {
      throw new IllegalArgumentException("No OpCode with value %s is defined.".formatted(value));
    }
    return opcodes[value] != null;
  }

  /**
   * isValid checks whether or not a given opcode index corresponds with a real opcode.
   *
   * @param opcode
   * @return
   */
  public boolean isValid(final OpCode opcode) {
    return isValid(opcode.getOpcode());
  }

  /**
   * Get opcode metadata per opcode long value.
   *
   * @param value opcode value.
   * @return an instance of {@link OpCodeData} corresponding to the numeric value.
   */
  public OpCodeData of(final int value) {
    if (value < 0 || value > 255) {
      throw new IllegalArgumentException("No OpCode with value %s is defined.".formatted(value));
    }
    //
    return Optional.ofNullable(opcodes[value]).orElse(OpCodeData.forNonOpCodes(value));
  }

  /**
   * Get opcode metadata per opcode mnemonic of type {@link OpCode}.
   *
   * @param code opcode mnemonic of type {@link OpCode}.
   * @return an instance of {@link OpCodeData} corresponding to mnemonic of type {@link OpCode}.
   */
  public OpCodeData of(final OpCode code) {
    //
    return Optional.ofNullable(opcodes[code.getOpcode()])
        .orElseThrow(
            () ->
                new IllegalArgumentException(
                    "No OpCode of mnemonic %s is defined.".formatted(code)));
  }

  /**
   * Get an iterator over the underlying opcode data.
   *
   * @return
   */
  public Iterable<OpCodeData> iterator() {
    return Arrays.asList(opcodes);
  }

  // Load opcodedata from yaml file.
  @SneakyThrows(IOException.class)
  private static List<OpCodeData> opCodesLocal(Fork fork) {
    final JsonConverter yamlConverter = JsonConverter.builder().enableYaml().build();

    final String yamlFileName = toCamelCase(fork) + "Opcodes.yml";

    final JsonNode rootNode =
        yamlConverter
            .getObjectMapper()
            .readTree(OpCodes.class.getClassLoader().getResourceAsStream(yamlFileName))
            .get("opcodes");

    final CollectionType typeReference =
        TypeFactory.defaultInstance().constructCollectionType(List.class, OpCodeData.class);

    SimpleModule module = new SimpleModule();
    module.addDeserializer(Billing.class, new BillingDeserializer(false));
    yamlConverter.getObjectMapper().registerModule(module);

    return yamlConverter.getObjectMapper().treeToValue(rootNode, typeReference);
  }
}
