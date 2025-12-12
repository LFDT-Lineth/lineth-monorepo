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

package net.consensys.linea.zktracer.module.hub.section.create;

import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.fragment.imc.oob.opcodes.create.CreateOobCall;
import net.consensys.linea.zktracer.module.hub.fragment.imc.oob.opcodes.create.ShanghaiCreateOobCall;
import net.consensys.linea.zktracer.module.hub.fragment.imc.oob.opcodes.create.XCreateOobCall;
import net.consensys.linea.zktracer.module.hub.signals.Exceptions;
import org.hyperledger.besu.evm.frame.MessageFrame;

public class ShanghaiCreateSection extends LondonCreateSection {
  public ShanghaiCreateSection(Hub hub, MessageFrame frame) {
    super(hub, frame);
  }

  @Override
  protected boolean maxCodeSizeExceptionalCreate(final short exceptions) {
    final boolean haltCreateSection = Exceptions.maxCodeSizeException(exceptions);
    if (haltCreateSection) {
      imcFragment.callOob(new XCreateOobCall());
    }
    return haltCreateSection;
  }

  @Override
  protected CreateOobCall createOobCall() {
    return new ShanghaiCreateOobCall();
  }
}
