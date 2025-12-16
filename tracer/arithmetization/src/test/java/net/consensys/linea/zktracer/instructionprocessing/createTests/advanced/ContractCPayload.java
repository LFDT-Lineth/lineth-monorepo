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

import java.util.Arrays;
import java.util.Collections;
import net.consensys.linea.testing.generated.ContractC;
import org.apache.tuweni.bytes.Bytes;
import org.web3j.abi.FunctionEncoder;
import org.web3j.abi.datatypes.Function;

public class ContractCPayload {

  public static Bytes storeInMap(int key, String add) {
    Function function =
        new Function(
            ContractC.FUNC_STOREINMAP,
            Arrays.asList(
                new org.web3j.abi.datatypes.generated.Uint256(key),
                new org.web3j.abi.datatypes.Address(160, add)),
            Collections.emptyList());
    return Bytes.fromHexStringLenient(FunctionEncoder.encode(function));
  }

  public static Bytes callBackCustomCreate2(String addCustomCreate2) {
    Function function =
        new Function(
            ContractC.FUNC_CALLBACKCUSTOMCREATE2,
            Arrays.asList(new org.web3j.abi.datatypes.Address(160, addCustomCreate2)),
            Collections.emptyList());
    return Bytes.fromHexStringLenient(FunctionEncoder.encode(function));
  }

  public static Bytes revertOnDemand() {
    Function function =
        new Function(ContractC.FUNC_REVERTONDEMAND, Arrays.asList(), Collections.emptyList());
    return Bytes.fromHexStringLenient(FunctionEncoder.encode(function));
  }

  public static Bytes selfDestructOnDemand() {
    Function function =
        new Function(ContractC.FUNC_SELFDESTRUCTONDEMAND, Arrays.asList(), Collections.emptyList());
    return Bytes.fromHexStringLenient(FunctionEncoder.encode(function));
  }
}
