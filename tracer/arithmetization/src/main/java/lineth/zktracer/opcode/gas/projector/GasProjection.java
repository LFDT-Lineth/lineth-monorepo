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

package lineth.zktracer.opcode.gas.projector;

import static com.google.common.base.Preconditions.*;
import static lineth.zktracer.Trace.WORD_SIZE;
import static org.hyperledger.besu.evm.internal.Words.*;

import lineth.zktracer.Fork;
import lineth.zktracer.module.mxp.MxpUtils;
import net.consensys.linea.zktracer.types.Range;
import org.hyperledger.besu.evm.frame.MessageFrame;
import org.hyperledger.besu.evm.gascalculator.GasCalculator;

public abstract class GasProjection {

  /**
   * Besu's {@code GasCalculator#memoryExpansionGasCost} is backed by a memory model limited to
   * {@code Integer.MAX_VALUE} bytes (its backing store is a Java array), so for an access beyond
   * that bound it short-circuits to a sentinel cost of {@code Long.MAX_VALUE} rather than the
   * actual (still finite, still <i>much smaller than {@code Long.MAX_VALUE}</i>) EVM memory cost.
   * The hub's own STP/MXP modules have no such limitation and compute the real cost, so relying on
   * Besu's sentinel here would desynchronize the traced {@code GAS_COST} from the STP/MXP-derived
   * columns. Recompute directly in that case.
   */
  static long memoryExpansionGasCost(GasCalculator gc, MessageFrame frame, long offset, long size) {
    final Range range = Range.fromOffsetAndSize(offset, size);
    if (range.besuOverflow()) {
      final long preWords = frame.memoryWordSize();
      final long postWords =
          range.isEmpty()
              ? preWords
              : Math.max(clampedAdd(clampedAdd(range.offset(), range.size()), 31) / 32, preWords);
      return MxpUtils.memoryCost(postWords) - MxpUtils.memoryCost(preWords);
    }
    return gc.memoryExpansionGasCost(frame, offset, size);
  }

  long linearCost(long costPerUnit, long size, long unit) {
    checkArgument(
        (unit == 1) || (unit == WORD_SIZE),
        "GasProjection.linearCost: unit must be 1 (per byte) or 32 (per word)");
    return clampedMultiply(costPerUnit, clampedAdd(size, unit - 1) / unit);
  }

  public long staticGas() {
    return 0;
  }

  public long expGas() {
    return 0;
  }

  public long memoryExpansion() {
    return 0;
  }

  public long accountAccess() {
    return 0;
  }

  public long accountCreation() {
    return 0;
  }

  public long transferValue() {
    return 0;
  }

  public long linearPerWord() {
    return 0;
  }

  public long linearPerByte() {
    return 0;
  }

  public long storageWarmth() {
    return 0;
  }

  public long sStoreValue() {
    return 0;
  }

  public long gasPaidOutOfPocket() {
    return 0;
  }

  public long stipend() {
    return 0;
  }

  public long deploymentCost() {
    return 0;
  }

  public long refund() {
    return 0;
  }

  public long messageSize() {
    return 0;
  }

  public long initCode() {
    return 0;
  }

  /**
   * Returns the value that will be checked against MXPX_THRESHOLD: that is: - For legacy MXP
   * (London, Paris, Shanghai): size == 0 ? 0 : offset + (size - 1) - For Cancun and after: size ==
   * 0 ? 0 : Max(size, offset)
   */
  public long mxpxOffset() {
    return 0;
  }

  /**
   * {@link GasProjection#upfrontGasCost()} computes the upfront gas cost of instructions, that is,
   * the gas cost that determines whether an <b>OUT_OF_GAS_EXCEPTION</b> occurred. This cost
   * purposefully <i>excludes</i> the gas paid "out of pocket" to child contexts in case of
   * <b>CALL</b>-type or <b>CREATE</b>-type instructions.
   *
   * @return
   */
  public final long upfrontGasCost() {
    return clampedAdd(gasCostExcludingDeploymentCost(), deploymentCost());
  }

  public final long gasCostExcludingDeploymentCost() {
    long cost = staticGas();
    cost = clampedAdd(cost, expGas());
    cost = clampedAdd(cost, memoryExpansion());
    cost = clampedAdd(cost, accountAccess());
    cost = clampedAdd(cost, accountCreation());
    cost = clampedAdd(cost, transferValue());
    cost = clampedAdd(cost, linearPerWord());
    cost = clampedAdd(cost, linearPerByte());
    cost = clampedAdd(cost, storageWarmth());
    cost = clampedAdd(cost, sStoreValue());
    cost = clampedAdd(cost, initCode());
    return cost;
  }

  public final boolean isMemoryExpansionFault(Fork fork) {
    return false;
  }
}
