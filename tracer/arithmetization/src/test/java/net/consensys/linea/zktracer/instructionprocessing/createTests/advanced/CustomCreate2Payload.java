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
package net.consensys.linea.zktracer.instructionprocessing.createTests.advanced;

import static net.consensys.linea.testing.generated.CustomCreate2.FUNC_ADVANCEDCREATESCENARIINESTEDCALLS;
import static net.consensys.linea.testing.generated.CustomCreate2.FUNC_ADVANCEDCREATESCENARIITRIGGEREDFROMROOT;

import java.util.Arrays;
import java.util.Collections;
import net.consensys.linea.testing.generated.CustomCreate2;
import org.apache.tuweni.bytes.Bytes;
import org.web3j.abi.FunctionEncoder;
import org.web3j.abi.datatypes.Function;

public class CustomCreate2Payload {

  public static Bytes storeSalt(String salt) {
    Function function =
        new Function(
            CustomCreate2.FUNC_STORESALT,
            Arrays.asList(
                new org.web3j.abi.datatypes.generated.Bytes32(
                    Bytes.fromHexStringLenient(salt).toArray())),
            Collections.emptyList());
    return Bytes.fromHexStringLenient(FunctionEncoder.encode(function));
  }

  public static Bytes storeInitCodeC(String initCodeC) {
    Function function =
        new Function(
            CustomCreate2.FUNC_STOREINITCODEC,
            Arrays.asList(
                new org.web3j.abi.datatypes.DynamicBytes(Bytes.fromHexString(initCodeC).toArray())),
            Collections.emptyList());
    return Bytes.fromHexStringLenient(FunctionEncoder.encode(function));
  }

  public static Bytes create2WithInitCodeC_withValueAndRevert() {
    Function function =
        new Function(
            CustomCreate2.FUNC_CREATE2WITHINITCODEC_WITHVALUEANDREVERT,
            Arrays.asList(),
            Collections.emptyList());
    return Bytes.fromHexStringLenient(FunctionEncoder.encode(function));
  }

  public static Bytes create2WithInitCodeC_noValueNoRevert() {
    Function function =
        new Function(
            CustomCreate2.FUNC_CREATE2WITHINITCODEC_NOVALUENOREVERT,
            Arrays.asList(),
            Collections.emptyList());
    return Bytes.fromHexStringLenient(FunctionEncoder.encode(function));
  }

  public static Bytes create2FourTimes_withRevertTrigger(boolean triggerRevert) {
    Function function =
        new Function(
            CustomCreate2.FUNC_CREATE2FOURTIMES_WITHREVERTTRIGGER,
            Arrays.asList(new org.web3j.abi.datatypes.Bool(triggerRevert)),
            Collections.emptyList());
    return Bytes.fromHexStringLenient(FunctionEncoder.encode(function));
  }

  public static Bytes create2WithStaticCall(boolean nested) {
    Function function =
        new Function(
            CustomCreate2.FUNC_CREATE2WITHSTATICCALL,
            Arrays.asList(new org.web3j.abi.datatypes.Bool(nested)),
            Collections.emptyList());
    return Bytes.fromHexStringLenient(FunctionEncoder.encode(function));
  }

  public static Bytes create2CallC_withRevertTrigger(boolean triggerRevert, boolean nested) {
    Function function =
        new Function(
            CustomCreate2.FUNC_CREATE2CALLC_WITHREVERTTRIGGER,
            Arrays.asList(
                new org.web3j.abi.datatypes.Bool(triggerRevert),
                new org.web3j.abi.datatypes.Bool(nested)),
            Collections.emptyList());
    return Bytes.fromHexStringLenient(FunctionEncoder.encode(function));
  }

  public static Bytes create2WithCallCtoCallback_noValue(boolean nested) {
    Function function =
        new Function(
            CustomCreate2.FUNC_CREATE2WITHCALLCTOCALLBACK_NOVALUE,
            Arrays.asList(new org.web3j.abi.datatypes.Bool(nested)),
            Collections.emptyList());
    return Bytes.fromHexStringLenient(FunctionEncoder.encode(function));
  }

  public static Bytes callCToModifyStorageAndSelfdestruct() {
    Function function =
        new Function(
            CustomCreate2.FUNC_CALLCTOMODIFYSTORAGEANDSELFDESTRUCT,
            Arrays.asList(),
            Collections.emptyList());
    return Bytes.fromHexStringLenient(FunctionEncoder.encode(function));
  }

  public static Bytes callMyself(Bytes executePayload, Boolean staticCall, int gas) {
    Function function =
        new Function(
            CustomCreate2.FUNC_CALLMYSELF,
            Arrays.asList(
                new org.web3j.abi.datatypes.DynamicBytes(executePayload.toArray()),
                new org.web3j.abi.datatypes.Bool(staticCall),
                new org.web3j.abi.datatypes.generated.Uint256(gas)),
            Collections.emptyList());
    return Bytes.fromHexStringLenient(FunctionEncoder.encode(function));
  }

  public static Bytes callContractC(Bytes executePayload, Boolean staticCall) {
    Function function =
        new Function(
            CustomCreate2.FUNC_CALLCONTRACTC,
            Arrays.asList(
                new org.web3j.abi.datatypes.DynamicBytes(executePayload.toArray()),
                new org.web3j.abi.datatypes.Bool(staticCall)),
            Collections.emptyList());
    return Bytes.fromHexStringLenient(FunctionEncoder.encode(function));
  }

  public static Bytes advancedCreateScenariiTriggeredFromRoot(String initCodeC, String salt) {
    Function function =
        new Function(
            FUNC_ADVANCEDCREATESCENARIITRIGGEREDFROMROOT,
            Arrays.asList(
                new org.web3j.abi.datatypes.DynamicBytes(Bytes.fromHexString(initCodeC).toArray()),
                new org.web3j.abi.datatypes.generated.Bytes32(
                    Bytes.fromHexStringLenient(salt).toArray())),
            Collections.emptyList());
    return Bytes.fromHexStringLenient(FunctionEncoder.encode(function));
  }

  public static Bytes advancedCreateScenariiNestedCalls(String initCodeC, String salt) {
    Function function =
        new Function(
            FUNC_ADVANCEDCREATESCENARIINESTEDCALLS,
            Arrays.asList(
                new org.web3j.abi.datatypes.DynamicBytes(Bytes.fromHexString(initCodeC).toArray()),
                new org.web3j.abi.datatypes.generated.Bytes32(
                    Bytes.fromHexStringLenient(salt).toArray())),
            Collections.emptyList());
    return Bytes.fromHexStringLenient(FunctionEncoder.encode(function));
  }
}
