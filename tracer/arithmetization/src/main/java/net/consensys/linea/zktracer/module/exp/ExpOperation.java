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

package net.consensys.linea.zktracer.module.exp;

import static com.google.common.base.Preconditions.*;
import static com.google.common.math.BigIntegerMath.log2;
import static java.lang.Math.min;
import static net.consensys.linea.zktracer.Trace.*;
import static net.consensys.linea.zktracer.module.hub.precompiles.ModexpMetadata.BASE_MIN_OFFSET;
import static net.consensys.linea.zktracer.types.Conversions.bigIntegerToBytes;

import java.math.BigInteger;
import java.math.RoundingMode;
import lombok.EqualsAndHashCode;
import lombok.Getter;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.container.ModuleOperation;
import net.consensys.linea.zktracer.module.hub.fragment.imc.exp.ExpCall;
import net.consensys.linea.zktracer.module.hub.fragment.imc.exp.ExplogExpCall;
import net.consensys.linea.zktracer.module.hub.fragment.imc.exp.ModexpLogExpCall;
import net.consensys.linea.zktracer.module.hub.precompiles.ModexpMetadata;
import net.consensys.linea.zktracer.types.EWord;
import org.apache.tuweni.bytes.Bytes;

@EqualsAndHashCode(onlyExplicitlyIncluded = true, callSuper = false)
@Getter
@Accessors(fluent = true)
public class ExpOperation extends ModuleOperation {
  @EqualsAndHashCode.Include ExpCall expCall;

  public ExpOperation(ExpCall expCall) {
    this.expCall = expCall;

    switch (expCall.expInstruction()) {
      case EXP_INST_EXPLOG -> {
        // done in the constructor of ExpLogExpCall
      }
      case EXP_INST_MODEXPLOG -> {
        final ModexpLogExpCall modexplogExpCall = (ModexpLogExpCall) expCall;
        // Extract inputs
        final ModexpMetadata modexpMetadata = modexplogExpCall.getModexpMetadata();
        final int bbsInt = modexpMetadata.normalizedBbsInt();
        final int ebsInt = modexpMetadata.normalizedEbsInt();
        checkArgument(
            modexpMetadata.callData().size() - BASE_MIN_OFFSET - bbsInt >= 0,
            "MODEXP call data unexpectedly short");
        final EWord rawLead = modexpMetadata.rawLeadingWord();
        final int cdsCutoff =
            Math.min(modexpMetadata.callData().size() - BASE_MIN_OFFSET - bbsInt, WORD_SIZE);
        final int ebsCutoff = Math.min(ebsInt, WORD_SIZE);
        final BigInteger leadLog =
            BigInteger.valueOf(LeadLogTrimLead.fromArgs(rawLead, cdsCutoff, ebsCutoff).leadLog());
        // Fill expCall
        modexplogExpCall.setRawLeadingWord(rawLead);
        modexplogExpCall.setCdsCutoff(cdsCutoff);
        modexplogExpCall.setEbsCutoff(ebsCutoff);
        modexplogExpCall.setLeadLog(leadLog);
      }
      default ->
          throw new IllegalArgumentException(
              "invalid EXP instruction: " + expCall.expInstruction());
    }
  }

  final void trace(Trace.Exp trace) {
    // Handle each case separately
    switch (expCall.expInstruction()) {
      case EXP_INST_EXPLOG -> {
        final ExplogExpCall call = (ExplogExpCall) expCall;
        trace
            .inst(EXP_INST_EXPLOG)
            .arg(call.exponent())
            .cds(0)
            .ebs(0)
            .res(Bytes.ofUnsignedLong(call.dynCost()))
            .validateRow();
      }
      case EXP_INST_MODEXPLOG -> {
        final ModexpLogExpCall call = (ModexpLogExpCall) expCall;
        trace
            .inst(EXP_INST_MODEXPLOG)
            .arg(call.getRawLeadingWord())
            .cds(call.getCdsCutoff())
            .ebs(call.getEbsCutoff())
            .res(bigIntegerToBytes(call.getLeadLog()))
            .validateRow();
      }
      default ->
          throw new IllegalArgumentException(
              "invalid EXP instruction: " + expCall.expInstruction());
    }
  }

  @Override
  protected int computeLineCount() {
    return 1;
  }

  public record LeadLogTrimLead(int leadLog, BigInteger trim) {
    public static LeadLogTrimLead fromArgs(EWord rawLead, int cdsCutoff, int ebsCutoff) {
      // min_cutoff
      final int minCutoff = min(cdsCutoff, ebsCutoff);

      BigInteger mask = new BigInteger("FF".repeat(minCutoff), 16);
      if (minCutoff < WORD_SIZE) {
        // 32 - minCutoff is the shift distance in bytes, but we need bits
        mask = mask.shiftLeft(8 * (WORD_SIZE - minCutoff));
      }

      // trim (keep only minCutoff bytes of rawLead)
      final BigInteger trim = rawLead.toUnsignedBigInteger().and(mask);

      // lead (keep only minCutoff bytes of rawLead and potentially pad to ebsCutoff with 0's)
      final BigInteger lead = trim.shiftRight(8 * (WORD_SIZE - ebsCutoff));

      // lead_log (same as EYP)
      final int leadLog = lead.signum() == 0 ? 0 : log2(lead, RoundingMode.FLOOR);

      return new LeadLogTrimLead(leadLog, trim);
    }
  }
}
