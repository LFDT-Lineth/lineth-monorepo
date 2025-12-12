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

package net.consensys.linea.zktracer.module.blockdata;

import static net.consensys.linea.zktracer.Trace.*;
import static net.consensys.linea.zktracer.types.Conversions.ZERO;
import static net.consensys.linea.zktracer.types.Conversions.booleanToBytes;

import lombok.Builder;
import lombok.Getter;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.module.euc.Euc;
import net.consensys.linea.zktracer.module.euc.EucOperation;
import net.consensys.linea.zktracer.module.wcp.Wcp;
import net.consensys.linea.zktracer.types.EWord;
import org.apache.tuweni.bytes.Bytes;

@Builder
@Getter
@Accessors(fluent = true)
public class BlockDataExoCall {

  @Builder.Default private final boolean wcpFlag = false;
  @Builder.Default private final boolean eucFlag = false;
  @Builder.Default private final int instruction = 0;
  @Builder.Default private final Bytes arg1Hi = Bytes.EMPTY;
  @Builder.Default private final Bytes arg1Lo = Bytes.EMPTY;
  @Builder.Default private final Bytes arg2Hi = Bytes.EMPTY;
  @Builder.Default private final Bytes arg2Lo = Bytes.EMPTY;
  // Results for wcp computations
  // Quotients for euc computations in trace
  @Builder.Default private final Bytes res = ZERO;

  public static BlockDataExoCall callToLT(final Wcp wcp, Bytes arg1, Bytes arg2) {

    final EWord arg1B32 = EWord.of(arg1);
    final EWord arg2B32 = EWord.of(arg2);

    return BlockDataExoCall.builder()
        .wcpFlag(true)
        .instruction(EVM_INST_LT)
        .arg1Hi(arg1B32.hi())
        .arg1Lo(arg1B32.lo())
        .arg2Hi(arg2B32.hi())
        .arg2Lo(arg2B32.lo())
        .res(booleanToBytes(wcp.callLT(arg1B32, arg2B32)))
        .build();
  }

  public static BlockDataExoCall callToGT(final Wcp wcp, Bytes arg1, Bytes arg2) {

    final EWord arg1B32 = EWord.of(arg1);
    final EWord arg2B32 = EWord.of(arg2);

    return BlockDataExoCall.builder()
        .wcpFlag(true)
        .instruction(EVM_INST_GT)
        .arg1Hi(arg1B32.hi())
        .arg1Lo(arg1B32.lo())
        .arg2Hi(arg2B32.hi())
        .arg2Lo(arg2B32.lo())
        .res(booleanToBytes(wcp.callGT(arg1B32, arg2B32)))
        .build();
  }

  public static BlockDataExoCall callToLEQ(final Wcp wcp, Bytes arg1, Bytes arg2) {

    final EWord arg1B32 = EWord.of(arg1);
    final EWord arg2B32 = EWord.of(arg2);

    return BlockDataExoCall.builder()
        .wcpFlag(true)
        .instruction(WCP_INST_LEQ)
        .arg1Hi(arg1B32.hi())
        .arg1Lo(arg1B32.lo())
        .arg2Hi(arg2B32.hi())
        .arg2Lo(arg2B32.lo())
        .res(booleanToBytes(wcp.callLEQ(arg1B32, arg2B32)))
        .build();
  }

  public static BlockDataExoCall callToGEQ(final Wcp wcp, Bytes arg1, Bytes arg2) {

    final EWord arg1B32 = EWord.of(arg1);
    final EWord arg2B32 = EWord.of(arg2);

    return BlockDataExoCall.builder()
        .wcpFlag(true)
        .instruction(WCP_INST_GEQ)
        .arg1Hi(arg1B32.hi())
        .arg1Lo(arg1B32.lo())
        .arg2Hi(arg2B32.hi())
        .arg2Lo(arg2B32.lo())
        .res(booleanToBytes(wcp.callGEQ(arg1B32, arg2B32)))
        .build();
  }

  public static BlockDataExoCall callToIsZero(final Wcp wcp, Bytes arg1) {

    final EWord arg1B32 = EWord.of(arg1);

    return BlockDataExoCall.builder()
        .wcpFlag(true)
        .instruction(EVM_INST_ISZERO)
        .arg1Hi(arg1B32.hi())
        .arg1Lo(arg1B32.lo())
        .res(booleanToBytes(wcp.callISZERO(arg1B32)))
        .build();
  }

  public static BlockDataExoCall callToEUC(final Euc euc, Bytes arg1, Bytes arg2) {

    final EWord arg1B32 = EWord.of(arg1);
    final EWord arg2B32 = EWord.of(arg2);

    EucOperation eucOperation = euc.callEUC(arg1B32, arg2B32);

    return BlockDataExoCall.builder()
        .eucFlag(true)
        .arg1Hi(arg1B32.hi())
        .arg1Lo(arg1B32.lo())
        .arg2Hi(arg2B32.hi())
        .arg2Lo(arg2B32.lo())
        .res(eucOperation.quotient())
        .build();
  }
}
