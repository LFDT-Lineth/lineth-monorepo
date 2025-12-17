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

package net.consensys.linea.zktracer.module.rlptxn.phaseSection;

import static net.consensys.linea.zktracer.module.rlputilsOld.Pattern.innerRlpSize;
import static net.consensys.linea.zktracer.types.Utils.rightPadToBytes16;
import static org.hyperledger.besu.ethereum.core.encoding.EncodingContext.BLOCK_BODY;
import static org.hyperledger.besu.ethereum.core.encoding.TransactionEncoder.encodeOpaqueBytes;

import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.rlpUtils.InstructionByteStringPrefix;
import net.consensys.linea.zktracer.module.rlpUtils.RlpUtils;
import net.consensys.linea.zktracer.module.rlpUtils.RlpUtilsCall;
import net.consensys.linea.zktracer.module.rlptxn.GenericTracedValue;
import net.consensys.linea.zktracer.types.TransactionProcessingMetadata;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.ethereum.core.Transaction;

public class GlobalPrefixPhaseSection extends PhaseSection {
  private final InstructionByteStringPrefix ltByteSizeCall;
  private final InstructionByteStringPrefix lxByteSizeCall;

  public GlobalPrefixPhaseSection(RlpUtils rlpUtils, GenericTracedValue tracedValues) {
    final Bytes besuRlpLt =
        encodeOpaqueBytes((Transaction) tracedValues.tx().getBesuTransaction(), BLOCK_BODY);
    final Bytes besuRlpLx = tracedValues.tx().getBesuTransaction().encodedPreimage();

    switch (tracedValues.tx().getBesuTransaction().getType()) {
      case FRONTIER -> {
        tracedValues.rlpLtByteSize(innerRlpSize(besuRlpLt.size()));
        tracedValues.rlpLxByteSize(innerRlpSize(besuRlpLx.size()));
      }
      case ACCESS_LIST, EIP1559 -> {
        // the innerRlp method already concatenate with the first byte "transaction  type"
        tracedValues.rlpLtByteSize(innerRlpSize(besuRlpLt.size() - 1));
        tracedValues.rlpLxByteSize(innerRlpSize(besuRlpLx.size() - 1));
      }
      default ->
          throw new IllegalStateException(
              "Transaction Type not supported: "
                  + tracedValues.tx().getBesuTransaction().getType());
    }

    final RlpUtilsCall rlpLtCall =
        new InstructionByteStringPrefix(tracedValues.rlpLtByteSize(), (byte) 0x00, true);
    ltByteSizeCall = (InstructionByteStringPrefix) rlpUtils.call(rlpLtCall);
    final RlpUtilsCall rlpLxCall =
        new InstructionByteStringPrefix(tracedValues.rlpLxByteSize(), (byte) 0x00, true);
    lxByteSizeCall = (InstructionByteStringPrefix) rlpUtils.call(rlpLxCall);
  }

  @Override
  protected void traceComputationsRows(
      Trace.Rlptxn trace, TransactionProcessingMetadata tx, GenericTracedValue tracedValues) {
    // First Computation Row: byte type prefix
    traceTransactionConstantValues(trace, tracedValues);
    trace
        .cmp(true)
        .lt(true)
        .lx(true)
        .limbConstructed(!tracedValues.type0())
        .pCmpLimb(
            tracedValues.type0() ? Bytes.EMPTY : rightPadToBytes16(Bytes.minimalBytes(tx.type())))
        .pCmpLimbSize(tracedValues.type0() ? 0 : 1);
    tracePostValues(trace, tracedValues);

    // Second Computation Row: RLP Prefix for Lt
    traceTransactionConstantValues(trace, tracedValues);
    ltByteSizeCall.traceRlpTxn(trace, tracedValues, true, false, false, 0);
    tracePostValues(trace, tracedValues);

    // Third Computation Row: RLP Prefix for Lx
    traceTransactionConstantValues(trace, tracedValues);
    lxByteSizeCall.traceRlpTxn(trace, tracedValues, false, true, false, 0);
    tracePostValues(trace, tracedValues);
  }

  @Override
  protected void traceIsPhaseX(Trace.Rlptxn trace) {
    trace.isRlpPrefix(true);
  }

  @Override
  public int lineCount() {
    return 4; // 1 for the txn, 1 for the byte type prefix, 1 for the RLP prefix of Lt, 1 for the
    // RLP prefix of Lx
  }
}
