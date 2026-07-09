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

import static net.consensys.linea.zktracer.Trace.*;
import static net.consensys.linea.zktracer.types.AddressUtils.isAddressWarm;
import static org.hyperledger.besu.evm.internal.Words.clampedAdd;

import lombok.RequiredArgsConstructor;
import net.consensys.linea.zktracer.Fork;
import net.consensys.linea.zktracer.module.mxp.MxpUtils;
import net.consensys.linea.zktracer.types.Bytecode;
import net.consensys.linea.zktracer.types.Range;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.evm.account.Account;
import org.hyperledger.besu.evm.frame.MessageFrame;
import org.hyperledger.besu.evm.gascalculator.GasCalculator;

@RequiredArgsConstructor
public class Call extends GasProjection {
  final Fork fork;
  final GasCalculator gc;
  private final MessageFrame frame;
  private final long stipend;
  private final Range callDataRange;
  private final Range returnAtRange;
  private final Wei value;
  private final Account recipient;
  private final Address to;

  public static Call invalid() {
    return new Call(null, null, null, 0, Range.empty(), Range.empty(), Wei.ZERO, null, null);
  }

  boolean isInvalid() {
    return this.frame == null;
  }

  @Override
  public long memoryExpansion() {
    if (this.isInvalid()) {
      return 0;
    }
    return Math.max(memoryExpansionGasCost(callDataRange), memoryExpansionGasCost(returnAtRange));
  }

  /**
   * Besu's {@code GasCalculator#memoryExpansionGasCost} is backed by a memory model limited to
   * {@code Integer.MAX_VALUE} bytes (its backing store is a Java array), so for a {@link Range}
   * beyond that bound it short-circuits to a sentinel cost of {@code Long.MAX_VALUE} rather than
   * the actual (still finite, still <i>much smaller than {@code Long.MAX_VALUE}</i>) EVM memory
   * cost. The hub's own STP/MXP modules have no such limitation and compute the real cost, so
   * relying on Besu's sentinel here would desynchronize {@code GAS_COST} from {@code
   * misc/STP_GAS_UPFRONT_GAS_COST} in the trace. Recompute directly in that case.
   */
  private long memoryExpansionGasCost(Range range) {
    if (range.besuOverflow()) {
      final long preWords = frame.memoryWordSize();
      final long postWords =
          range.isEmpty()
              ? preWords
              : Math.max(clampedAdd(clampedAdd(range.offset(), range.size()), 31) / 32, preWords);
      return MxpUtils.memoryCost(postWords) - MxpUtils.memoryCost(preWords);
    }
    return gc.memoryExpansionGasCost(frame, range.offset(), range.size());
  }

  @Override
  public long mxpxOffset() {
    if (this.isInvalid()) {
      return 0;
    }
    return Math.max(
        callDataRange.isEmpty() ? 0 : Math.max(callDataRange.offset(), callDataRange.size()),
        returnAtRange.isEmpty() ? 0 : Math.max(returnAtRange.offset(), returnAtRange.size()));
  }

  @Override
  public long accountAccess() {
    if (this.isInvalid()) {
      return 0;
    }

    final boolean isCallTypeInstruction =
        frame.getCurrentOperation().getName().equals("CALL")
            || frame.getCurrentOperation().getName().equals("CALLCODE")
            || frame.getCurrentOperation().getName().equals("DELEGATECALL")
            || frame.getCurrentOperation().getName().equals("STATICCALL");
    final boolean currentTargetWarmth = isAddressWarm(fork, frame, to);

    if (!isCallTypeInstruction) {
      return currentTargetWarmth ? GAS_CONST_G_WARM_ACCESS : GAS_CONST_G_COLD_ACCOUNT_ACCESS;
    }

    // beyond this point: CALL-type instruction
    final Account target = frame.getWorldUpdater().get(to);
    if (target == null) {
      return currentTargetWarmth ? GAS_CONST_G_WARM_ACCESS : GAS_CONST_G_COLD_ACCOUNT_ACCESS;
    }

    // beyond this point: to address exists in the state
    final Bytecode targetCode = new Bytecode(target.getCode());
    if (!targetCode.isDelegated()) {
      return currentTargetWarmth ? GAS_CONST_G_WARM_ACCESS : GAS_CONST_G_COLD_ACCOUNT_ACCESS;
    }

    // beyond this point: to address is delegated
    final Address delegateAddress = targetCode.getDelegateAddress().orElseThrow();
    final boolean isSelfDelegated = delegateAddress.getBytes().equals(to.getBytes());
    if (isSelfDelegated) {
      return (currentTargetWarmth ? GAS_CONST_G_WARM_ACCESS : GAS_CONST_G_COLD_ACCOUNT_ACCESS)
          + GAS_CONST_G_WARM_ACCESS;
    }

    // beyond this point: to address is delegated but not to itself
    final boolean currentDelegateWarmth = isAddressWarm(fork, frame, delegateAddress);
    return (currentTargetWarmth ? GAS_CONST_G_WARM_ACCESS : GAS_CONST_G_COLD_ACCOUNT_ACCESS)
        + (currentDelegateWarmth ? GAS_CONST_G_WARM_ACCESS : GAS_CONST_G_COLD_ACCOUNT_ACCESS);
  }

  @Override
  public long accountCreation() {
    if (this.isInvalid()) {
      return 0;
    }

    if ((recipient == null || recipient.isEmpty()) && !value.isZero()) {
      return GAS_CONST_G_NEW_ACCOUNT;
    } else {
      return 0L;
    }
  }

  @Override
  public long transferValue() {
    if (this.isInvalid()) {
      return 0;
    }

    if (value.isZero()) {
      return 0L;
    } else {
      return GAS_CONST_G_CALL_VALUE;
    }
  }

  @Override
  public long gasPaidOutOfPocket() {
    if (this.isInvalid()) {
      return 0;
    }

    final long upfrontGasCost =
        memoryExpansion() + accountAccess() + accountCreation() + transferValue();
    if (upfrontGasCost > frame.getRemainingGas()) {
      return 0L;
    }

    final long remaining = frame.getRemainingGas() - upfrontGasCost;
    final long sixtyThreeSixtyFourthsOfRemaining = remaining - remaining / 64;
    return Math.min(sixtyThreeSixtyFourthsOfRemaining, stipend);
  }

  @Override
  public long stipend() {
    if (this.isInvalid() || this.value.isZero()) {
      return 0;
    }

    return GAS_CONST_G_CALL_STIPEND;
  }
}
