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

import static org.hyperledger.besu.evm.internal.Words.clampedToLong;

import net.consensys.linea.zktracer.Trace;
import org.hyperledger.besu.evm.frame.MessageFrame;
import org.hyperledger.besu.evm.gascalculator.GasCalculator;

public final class Log extends GasProjection {
  final GasCalculator gc;
  private final MessageFrame frame;
  private long offset = 0;
  private long size = 0;
  private final int numTopics;

  public Log(GasCalculator gc, MessageFrame frame, int numTopics) {
    this.gc = gc;
    this.frame = frame;
    this.numTopics = numTopics;
    if (frame.stackSize() > 1) {
      this.offset = clampedToLong(frame.getStackItem(0));
      this.size = clampedToLong(frame.getStackItem(1));
    }
  }

  @Override
  public long staticGas() {
    switch (numTopics) {
      case 0 -> {
        return Trace.GAS_CONST_G_LOG;
      }
      case 1 -> {
        return Trace.GAS_CONST_G_LOG + Trace.GAS_CONST_G_LOG_TOPIC;
      }
      case 2 -> {
        return Trace.GAS_CONST_G_LOG + 2 * Trace.GAS_CONST_G_LOG_TOPIC;
      }
      case 3 -> {
        return Trace.GAS_CONST_G_LOG + 3 * Trace.GAS_CONST_G_LOG_TOPIC;
      }
      case 4 -> {
        return Trace.GAS_CONST_G_LOG + 4 * Trace.GAS_CONST_G_LOG_TOPIC;
      }
      default -> throw new IllegalStateException("Unexpected value: " + numTopics);
    }
  }

  @Override
  public long memoryExpansion() {
    return gc.memoryExpansionGasCost(frame, offset, size);
  }

  @Override
  public long mxpxOffset() {
    return size == 0 ? 0 : Math.max(offset, size);
  }

  @Override
  public long linearPerByte() {
    return linearCost(Trace.GAS_CONST_G_LOG_DATA, size, 1);
  }
}
