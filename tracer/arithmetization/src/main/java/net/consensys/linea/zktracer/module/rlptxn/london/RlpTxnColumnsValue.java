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

package net.consensys.linea.zktracer.module.rlptxn.london;

import java.math.BigInteger;
import org.apache.tuweni.bytes.Bytes;

class RlpTxnColumnsValue {
  Bytes acc1;
  Bytes acc2;
  int accByteSize;
  int accessTupleByteSize;
  Bytes addrHi;
  Bytes addrLo;
  boolean bit;
  int bitAcc;
  byte byte1;
  byte byte2;
  int counter;
  BigInteger dataHi;
  BigInteger dataLo;
  int dataGasCost;
  boolean depth1;
  boolean depth2;
  boolean phaseEnd;
  int indexData;
  int indexLt;
  int indexLx;
  Bytes input1;
  Bytes input2;
  boolean lcCorrection;
  boolean isPrefix;
  Bytes limb;
  boolean limbConstructed;
  boolean lt;
  boolean lx;
  int nBytes;
  int nbAddr;
  int nbSto;
  int nbStoPerAddr;
  int nStep;
  int phase;
  int phaseByteSize;
  BigInteger power;
  int rlpLtByteSize;
  int rlpLxByteSize;
  boolean requiresEvmExecution;
  int absTxNum;
  int codeFragmentIndex;
  int txType;

  void partialReset(int phase, int numberStep, boolean lt, boolean lx) {
    this.phase = phase;
    this.nStep = numberStep;
    this.lt = lt;
    this.lx = lx;

    // Set to default local values
    this.limbConstructed = false;
    this.acc1 = Bytes.of(0);
    this.acc2 = Bytes.of(0);
    this.accByteSize = 0;
    this.bit = false;
    this.bitAcc = 0;
    this.byte1 = 0;
    this.byte2 = 0;
    this.counter = 0;
    this.depth1 = false;
    this.depth2 = false;
    this.phaseEnd = false;
    this.input1 = Bytes.of(0);
    this.input2 = Bytes.of(0);
    this.lcCorrection = false;
    this.isPrefix = false;
    this.limb = Bytes.of(0);
    this.nBytes = 0;
    this.power = BigInteger.ZERO;
  }

  void resetDataHiLo() {
    this.dataHi = BigInteger.ZERO;
    this.dataLo = BigInteger.ZERO;
  }
}
