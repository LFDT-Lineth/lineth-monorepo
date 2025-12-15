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

package net.consensys.linea.zktracer.module.hub.signals;

import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.apache.tuweni.units.bigints.UInt256;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Wei;

/** Records the aborting conditions that may happen during a CALL or a CREATE. */
@Getter
@NoArgsConstructor
@Accessors(fluent = true)
public final class AbortingConditions {
  private boolean callStackOverflow;
  private boolean insufficientBalance;
  private boolean maxNonceReached;

  /**
   * @param callStackOverflow too many nested contexts
   * @param insufficientBalance trying to give more ETH than the caller has
   * @param maxNonceReached the nonce of the creator account is too high (EIP-2681)
   */
  public AbortingConditions(
      boolean callStackOverflow, boolean insufficientBalance, boolean maxNonceReached) {
    this.callStackOverflow = callStackOverflow;
    this.insufficientBalance = insufficientBalance;
    this.maxNonceReached = maxNonceReached;
  }

  public void reset() {
    callStackOverflow = false;
    insufficientBalance = false;
    maxNonceReached = false;
  }

  public void prepare(Hub hub) {
    callStackOverflow = hub.callStack().wouldOverflow();
    if (this.callStackOverflow) {
      return;
    }

    final OpCode currentOp = hub.opCode();

    insufficientBalance =
        switch (currentOp) {
          case CALL, CALLCODE -> {
            final Address myAddress = hub.currentFrame().accountAddress();
            final Wei myBalance = hub.messageFrame().getWorldUpdater().get(myAddress).getBalance();
            final Wei value = Wei.of(UInt256.fromBytes(hub.messageFrame().getStackItem(2)));

            yield value.greaterThan(myBalance);
          }
          case CREATE, CREATE2 -> {
            final Address myAddress = hub.currentFrame().accountAddress();
            final Wei myBalance = hub.messageFrame().getWorldUpdater().get(myAddress).getBalance();
            final Wei value = Wei.of(UInt256.fromBytes(hub.messageFrame().getStackItem(0)));

            yield value.greaterThan(myBalance);
          }
          default -> false;
        };

    if (insufficientBalance) {
      return;
    }

    maxNonceReached =
        switch (currentOp) {
          case CREATE, CREATE2 -> {
            final Address myAddress = hub.currentFrame().accountAddress();
            final long creatorNonce =
                hub.messageFrame().getWorldUpdater().get(myAddress).getNonce();
            // The nonce is a (signed) long for BESU so EIP2681_MAX_NONCE == -1
            yield creatorNonce == -1;
          }

          default -> false;
        };
  }

  public AbortingConditions snapshot() {
    return new AbortingConditions(callStackOverflow, insufficientBalance, maxNonceReached);
  }

  public boolean none() {
    return !this.any();
  }

  public boolean any() {
    return callStackOverflow || insufficientBalance || maxNonceReached;
  }
}
