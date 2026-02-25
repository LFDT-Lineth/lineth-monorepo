/*
 * Copyright ConsenSys Inc.
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

package net.consensys.linea.zktracer.module.hub.fragment.account;

import static net.consensys.linea.zktracer.Trace.LLARGE;
import static net.consensys.linea.zktracer.types.AddressUtils.hiPart;
import static net.consensys.linea.zktracer.types.AddressUtils.loPart;

import lombok.RequiredArgsConstructor;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.fragment.TraceSubFragment;
import net.consensys.linea.zktracer.module.hub.transients.OperationAncillaries;
import net.consensys.linea.zktracer.opcode.OpCodeData;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.Bytes32;
import org.hyperledger.besu.crypto.Hash;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.evm.frame.MessageFrame;

@RequiredArgsConstructor
public class RlpAddrSubFragment implements TraceSubFragment {
  private final short recipe;
  private final Address deploymentAddress;
  private final Bytes32 salt;
  private final Bytes32 keccak;

  public static RlpAddrSubFragment makeFragment(Hub hub, Address deploymentAddress) {
    final OpCodeData opCode = hub.opCodeData();
    switch (opCode.mnemonic()) {
      case CREATE2 -> {
        final MessageFrame frame = hub.currentFrame().frame();
        final Bytes32 salt = Bytes32.leftPad(frame.getStackItem(3));
        final Bytes initCode = OperationAncillaries.initCode(frame, opCode);
        final Bytes32 hash = Hash.keccak256(initCode);
        hub.rlpAddr().callRlpAddrCreate2(frame, salt, hash);
        return new RlpAddrSubFragment((short) 2, deploymentAddress, salt, hash);
      }
      case CREATE -> {
        hub.rlpAddr().callRlpAddrCreate();
        return new RlpAddrSubFragment((short) 1, deploymentAddress, Bytes32.ZERO, Bytes32.ZERO);
      }
      default -> throw new IllegalStateException("Unexpected value: " + opCode);
    }
  }

  @Override
  public Trace.Hub traceHub(Trace.Hub trace) {
    return trace
        .pAccountRlpaddrFlag(true)
        .pAccountRlpaddrRecipe(recipe)
        .pAccountRlpaddrDepAddrHi(hiPart(deploymentAddress))
        .pAccountRlpaddrDepAddrLo(loPart(deploymentAddress))
        .pAccountRlpaddrSaltHi(salt.slice(0, LLARGE))
        .pAccountRlpaddrSaltLo(salt.slice(LLARGE, LLARGE))
        .pAccountRlpaddrKecHi(keccak.slice(0, LLARGE))
        .pAccountRlpaddrKecLo(keccak.slice(LLARGE, LLARGE));
  }
}
