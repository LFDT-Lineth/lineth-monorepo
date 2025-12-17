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

import static net.consensys.linea.zktracer.Trace.WORD_SIZE;
import static org.hyperledger.besu.evm.internal.Words.clampedToLong;

import net.consensys.linea.zktracer.Trace;
import org.hyperledger.besu.evm.frame.MessageFrame;
import org.hyperledger.besu.evm.gascalculator.GasCalculator;

public final class Create2 extends GasProjection {
  final GasCalculator gc;
  private final MessageFrame frame;
  private long initCodeOffset = 0;
  private long initCodeLength = 0;

  public Create2(GasCalculator gc, MessageFrame frame) {
    this.gc = gc;
    this.frame = frame;
    if (frame.stackSize() > 2) {
      this.initCodeOffset = clampedToLong(frame.getStackItem(1));
      this.initCodeLength = clampedToLong(frame.getStackItem(2));
    }
  }

  @Override
  public long staticGas() {
    return Trace.GAS_CONST_G_CREATE;
  }

  @Override
  public long memoryExpansion() {
    return gc.memoryExpansionGasCost(frame, initCodeOffset, initCodeLength);
  }

  @Override
  public long initCode() {
    return gc.initcodeCost((int) initCodeLength);
  }

  @Override
  public long mxpxOffset() {
    return initCodeLength == 0 ? 0 : Math.max(initCodeOffset, initCodeLength);
  }

  @Override
  public long linearPerWord() {
    return linearCost(Trace.GAS_CONST_G_KECCAK_256_WORD, initCodeLength, WORD_SIZE);
  }

  @Override
  public long gasPaidOutOfPocket() {
    long currentGas = frame.getRemainingGas();
    long upfrontGasCost = this.upfrontGasCost();

    if (upfrontGasCost > currentGas) {
      return 0;
    }

    final long gasMinusUpfrontGasCost = currentGas - upfrontGasCost;

    return gasMinusUpfrontGasCost - gasMinusUpfrontGasCost / 64;
  }

  @Override
  public long messageSize() {
    return this.initCodeLength;
  }
}
