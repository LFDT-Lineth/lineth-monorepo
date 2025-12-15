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

import org.apache.tuweni.units.bigints.UInt256;
import org.hyperledger.besu.evm.account.Account;
import org.hyperledger.besu.evm.frame.MessageFrame;
import org.hyperledger.besu.evm.gascalculator.GasCalculator;

public final class SStore extends GasProjection {
  final GasCalculator gc;
  private final MessageFrame frame;
  private UInt256 key = UInt256.ZERO;
  private UInt256 originalValue = UInt256.ZERO;
  private UInt256 currentValue = UInt256.ZERO;
  private UInt256 newValue = UInt256.ZERO;

  public SStore(GasCalculator gc, MessageFrame frame) {
    this.gc = gc;
    this.frame = frame;
    if (frame.stackSize() > 1) {
      this.key = UInt256.fromBytes(frame.getStackItem(0));
      final Account account = frame.getWorldUpdater().get(frame.getRecipientAddress());
      if (account == null) {
        return;
      }

      this.originalValue = account.getOriginalStorageValue(key);
      this.currentValue = account.getStorageValue(key);
      this.newValue = UInt256.fromBytes(frame.getStackItem(1));
    }
  }

  @Override
  public long storageWarmth() {
    if (frame.getWarmedUpStorage().contains(frame.getRecipientAddress(), key)) {
      return 0L;
    } else {
      return GAS_CONST_G_COLD_SLOAD;
    }
  }

  @Override
  public long sStoreValue() {
    if (newValue.equals(currentValue) || !originalValue.equals(currentValue)) {
      return GAS_CONST_G_WARM_ACCESS;
    } else {
      return originalValue.isZero() ? GAS_CONST_G_SSET : GAS_CONST_G_SRESET;
    }
  }

  @Override
  public long refund() {

    if (currentValue.equals(newValue)) {
      return 0;
    }
    // beyond this point, v ≠ v'

    if (originalValue.equals(currentValue)) {
      return newValue.isZero() ? REFUND_CONST_R_SCLEAR : 0;
    }
    // beyond this point, v ≠ v' and v0 ≠ v

    long rDirtyClear = 0;
    if (!originalValue.isZero() && currentValue.isZero()) {
      rDirtyClear = -REFUND_CONST_R_SCLEAR;
    }
    if (!originalValue.isZero() && newValue.isZero()) {
      rDirtyClear = REFUND_CONST_R_SCLEAR;
    }

    long rDirtyReset = 0;
    if (originalValue.equals(newValue) && originalValue.isZero()) {
      rDirtyReset = GAS_CONST_G_SSET - GAS_CONST_G_WARM_ACCESS;
    }
    if (originalValue.equals(newValue) && !originalValue.isZero()) {
      rDirtyReset = GAS_CONST_G_SRESET - GAS_CONST_G_WARM_ACCESS;
    }

    return rDirtyClear + rDirtyReset;
  }
}
