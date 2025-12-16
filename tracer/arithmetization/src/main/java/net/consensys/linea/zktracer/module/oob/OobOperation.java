/*
 * Copyright ConsenSys AG.
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

package net.consensys.linea.zktracer.module.oob;

import static com.google.common.math.BigIntegerMath.log2;
import static java.lang.Math.min;
import static net.consensys.linea.zktracer.Trace.WORD_SIZE;
import static net.consensys.linea.zktracer.module.hub.precompiles.modexpMetadata.ModexpMetadata.BASE_MIN_OFFSET;
import static net.consensys.linea.zktracer.types.Utils.rightPadTo;

import java.math.BigInteger;
import java.math.RoundingMode;
import lombok.EqualsAndHashCode;
import lombok.Getter;
import lombok.Setter;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.container.ModuleOperation;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.fragment.imc.oob.OobCall;
import net.consensys.linea.zktracer.module.hub.precompiles.modexpMetadata.ModexpMetadata;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.evm.frame.MessageFrame;

@Getter
@Accessors(fluent = true)
@EqualsAndHashCode(onlyExplicitlyIncluded = true, callSuper = false)
public class OobOperation extends ModuleOperation {
  @EqualsAndHashCode.Include @Setter public OobCall oobCall;

  public int ctMax() {
    return oobCall.ctMax();
  }

  public int nRows() {
    return ctMax() + 1;
  }

  public OobOperation(OobCall oobCall, final Hub hub, final MessageFrame frame) {
    this.oobCall = oobCall;
    oobCall.setInputData(frame, hub);
  }

  // Support method for MODEXP
  public static int computeExponentLog(ModexpMetadata metadata, int cds) {
    final short bbs = metadata.normalizedBbsInt();
    final short ebs = metadata.normalizedEbsInt();
    return computeExponentLog(metadata, cds, bbs, ebs);
  }

  public static int computeExponentLog(ModexpMetadata modexpMetadata, int cds, int bbs, int ebs) {
    return computeExponentLog(
        modexpMetadata.callData(), modexpMetadata.getLeadLogByteMultiplier(), cds, bbs, ebs);
  }

  public static int computeExponentLog(Bytes callData, int multiplier, int cds, int bbs, int ebs) {
    // pad callData to 96 + bbs + ebs
    final Bytes paddedCallData =
        cds < BASE_MIN_OFFSET + bbs + ebs
            ? rightPadTo(callData, BASE_MIN_OFFSET + bbs + ebs)
            : callData;

    final BigInteger leadingBytesOfExponent =
        paddedCallData.slice(BASE_MIN_OFFSET + bbs, min(ebs, WORD_SIZE)).toUnsignedBigInteger();

    final int bitContribution =
        (leadingBytesOfExponent.signum() != 0)
            ? log2(leadingBytesOfExponent, RoundingMode.FLOOR)
            : 0;
    final int byteContribution = (ebs > WORD_SIZE) ? multiplier * (ebs - WORD_SIZE) : 0;

    return bitContribution + byteContribution;
  }

  @Override
  protected int computeLineCount() {
    return nRows();
  }
}
