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

package net.consensys.linea.zktracer.opcode.gas.projector;

import static net.consensys.linea.zktracer.Trace.GAS_CONST_G_COLD_SLOAD;
import static net.consensys.linea.zktracer.Trace.GAS_CONST_G_WARM_ACCESS;

import org.apache.tuweni.units.bigints.UInt256;
import org.hyperledger.besu.evm.frame.MessageFrame;
import org.hyperledger.besu.evm.gascalculator.GasCalculator;

public final class SLoad extends GasProjection {
  final GasCalculator gc;
  private final MessageFrame frame;
  private UInt256 key = null;

  public SLoad(GasCalculator gc, MessageFrame frame) {
    this.gc = gc;
    this.frame = frame;
    if (frame.stackSize() > 0) {
      this.key = UInt256.fromBytes(frame.getStackItem(0));
    }
  }

  @Override
  public long storageWarmth() {
    if (key == null) {
      return 0;
    } else {
      if (frame.getWarmedUpStorage().contains(frame.getRecipientAddress(), key)) {
        return GAS_CONST_G_WARM_ACCESS;
      } else {
        return GAS_CONST_G_COLD_SLOAD;
      }
    }
  }
}
