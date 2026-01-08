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

package net.consensys.linea.zktracer.module.hub.fragment.imc.exp;

import static net.consensys.linea.zktracer.Trace.EXP_INST_MODEXPLOG;
import static net.consensys.linea.zktracer.types.Conversions.bigIntegerToBytes;

import java.math.BigInteger;
import lombok.EqualsAndHashCode;
import lombok.Getter;
import lombok.RequiredArgsConstructor;
import lombok.Setter;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.hub.precompiles.ModexpMetadata;
import net.consensys.linea.zktracer.types.EWord;
import org.apache.tuweni.bytes.Bytes;

@Setter
@Getter
@RequiredArgsConstructor
@EqualsAndHashCode(onlyExplicitlyIncluded = true, callSuper = false)
public class ModexpLogExpCall implements ExpCall {
  final ModexpMetadata modexpMetadata;
  @EqualsAndHashCode.Include EWord rawLeadingWord;
  @EqualsAndHashCode.Include int cdsCutoff;
  @EqualsAndHashCode.Include int ebsCutoff;
  @EqualsAndHashCode.Include BigInteger leadLog;

  @Override
  public int expInstruction() {
    return EXP_INST_MODEXPLOG;
  }

  @Override
  public int compareTo(ExpCall op2) {
    final ModexpLogExpCall o2 = (ModexpLogExpCall) op2;

    final int cdsCutoffComp = Integer.compare(cdsCutoff, o2.getCdsCutoff());
    if (cdsCutoffComp != 0) {
      return cdsCutoffComp;
    }
    final int ebsCutoffComp = Integer.compare(ebsCutoff, o2.getEbsCutoff());
    if (ebsCutoffComp != 0) {
      return ebsCutoffComp;
    }
    final int leadLogComp = leadLog.compareTo(o2.getLeadLog());
    if (leadLogComp != 0) {
      return leadLogComp;
    }

    return rawLeadingWord.compareTo(o2.getRawLeadingWord());
  }

  @Override
  public Trace.Hub traceHub(Trace.Hub trace) {
    return trace
        .pMiscExpFlag(true)
        .pMiscExpInst(EXP_INST_MODEXPLOG)
        .pMiscExpData1(rawLeadingWord.hi())
        .pMiscExpData2(rawLeadingWord.lo())
        .pMiscExpData3(Bytes.ofUnsignedShort(cdsCutoff))
        .pMiscExpData4(Bytes.ofUnsignedShort(ebsCutoff))
        .pMiscExpData5(bigIntegerToBytes(leadLog));
  }

  public String toString() {
    return "MODEXPLOG("
        + rawLeadingWord.toString()
        + ", "
        + cdsCutoff
        + ", "
        + ebsCutoff
        + ")="
        + leadLog.toString();
  }
}
