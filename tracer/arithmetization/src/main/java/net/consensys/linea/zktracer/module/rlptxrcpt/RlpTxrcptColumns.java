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

package net.consensys.linea.zktracer.module.rlptxrcpt;

import static net.consensys.linea.zktracer.Trace.RLP_RCPT_SUBPHASE_ID_TOPIC_DELTA;
import static net.consensys.linea.zktracer.Trace.Rlptxrcpt.SUBPHASE_ID_WEIGHT_DEPTH;
import static net.consensys.linea.zktracer.Trace.Rlptxrcpt.SUBPHASE_ID_WEIGHT_IS_OD;
import static net.consensys.linea.zktracer.Trace.Rlptxrcpt.SUBPHASE_ID_WEIGHT_IS_OT;
import static net.consensys.linea.zktracer.Trace.Rlptxrcpt.SUBPHASE_ID_WEIGHT_IS_PREFIX;
import static net.consensys.linea.zktracer.types.Conversions.booleanToInt;

import java.math.BigInteger;

import org.apache.tuweni.bytes.Bytes;

class RlpTxrcptColumns {
  int absTxNum;
  int absLogNumMax;
  Bytes acc1;
  Bytes acc2;
  Bytes acc3;
  Bytes acc4;
  int accSize;
  boolean bit;
  int bitAcc;
  byte byte1;
  byte byte2;
  byte byte3;
  byte byte4;
  int counter;
  boolean depth1;
  int index;
  int indexLocal;
  Bytes input1;
  Bytes input2;
  Bytes input3;
  Bytes input4;
  boolean isData;
  boolean isPrefix;
  boolean isTopic;
  boolean lcCorrection;
  Bytes limb;
  boolean limbConstructed;
  int localSize;
  int logEntrySize;
  int nBytes;
  int nStep;
  int phase;
  boolean phaseEnd;
  int phaseSize;
  BigInteger power;
  int txrcptSize;

  void partialReset(int phase, int nStep) {
    this.phase = phase;
    this.nStep = nStep;

    // Set to default local values.
    this.acc1 = Bytes.ofUnsignedShort(0);
    this.acc2 = Bytes.ofUnsignedShort(0);
    this.acc3 = Bytes.ofUnsignedShort(0);
    this.acc4 = Bytes.ofUnsignedShort(0);
    this.accSize = 0;
    this.bit = false;
    this.bitAcc = 0;
    this.byte1 = 0;
    this.byte2 = 0;
    this.byte3 = 0;
    this.byte4 = 0;
    this.counter = 0;
    this.depth1 = false;
    this.input1 = Bytes.ofUnsignedShort(0);
    this.input2 = Bytes.ofUnsignedShort(0);
    this.input3 = Bytes.ofUnsignedShort(0);
    this.input4 = Bytes.ofUnsignedShort(0);
    this.isData = false;
    this.isPrefix = false;
    this.isTopic = false;
    this.lcCorrection = false;
    this.limb = Bytes.ofUnsignedShort(0);
    this.limbConstructed = false;
    this.nBytes = 0;
    this.phaseEnd = false;
    this.power = BigInteger.valueOf(0);
  }

  public int getPhaseId() {
    return this.phase
        + SUBPHASE_ID_WEIGHT_IS_PREFIX * booleanToInt(this.isPrefix)
        + SUBPHASE_ID_WEIGHT_IS_OT * booleanToInt(this.isTopic)
        + SUBPHASE_ID_WEIGHT_IS_OD * booleanToInt(this.isData)
        + SUBPHASE_ID_WEIGHT_DEPTH * booleanToInt(this.depth1)
        + RLP_RCPT_SUBPHASE_ID_TOPIC_DELTA * booleanToInt(this.isTopic) * this.indexLocal;
  }
}
