package net.consensys.linea.zktracer.bytestheta;

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

import static org.assertj.core.api.Assertions.assertThat;

import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.Bytes32;
import org.apache.tuweni.units.bigints.UInt256;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;

/**
 * Test class related to {@link BaseTheta} data structure, which is an extension of {@link
 * BytesArray}, with support for high and low bytes' manipulation.
 */
@ExtendWith(UnitTestWatcher.class)
public class BaseThetaTest extends TracerTestBase {
  @Test
  public void baseThetaTest() {
    Bytes firstByte = Bytes.fromHexString("0x000000000000000a");
    Bytes secondByte = Bytes.fromHexString("0x000000000000000b");
    Bytes thirdByte = Bytes.fromHexString("0x000000000000000c");
    Bytes fourthByte = Bytes.fromHexString("0x000000000000000d");
    Bytes32 bytes32 = Bytes32.wrap(Bytes.concatenate(firstByte, secondByte, thirdByte, fourthByte));

    BaseTheta baseTheta = BaseTheta.fromBytes32(bytes32);
    Bytes32 expectedBytes32 =
        Bytes32.wrap(Bytes.concatenate(fourthByte, thirdByte, secondByte, firstByte));

    assertThat(baseTheta.getBytes32()).isEqualTo(expectedBytes32);

    Bytes expectedLow = Bytes.concatenate(thirdByte, fourthByte);
    Bytes expectedHigh = Bytes.concatenate(firstByte, secondByte);

    assertThat(baseTheta.getLow()).isEqualTo(expectedLow);
    assertThat(baseTheta.getHigh()).isEqualTo(expectedHigh);

    assertThat(baseTheta.get(0)).isEqualTo(fourthByte);
    assertThat(baseTheta.get(1)).isEqualTo(thirdByte);
    assertThat(baseTheta.get(2)).isEqualTo(secondByte);
    assertThat(baseTheta.get(3)).isEqualTo(firstByte);
  }

  @Test
  public void baseBytesTest() {
    Bytes32 bytes32 =
        Bytes32.fromHexString("0x000000000000000a000000000000000b000000000000000c000000000000000d");
    BaseBytes baseBytes = BaseBytes.fromBytes32(bytes32);
    Bytes expectedHigh = Bytes.fromHexString("0x000000000000000a000000000000000b");
    Bytes expectedLow = Bytes.fromHexString("0x000000000000000c000000000000000d");

    assertThat(baseBytes.getBytes32()).isEqualTo(bytes32);
    assertThat(baseBytes.getLow()).isEqualTo(expectedLow);
    assertThat(baseBytes.getHigh()).isEqualTo(expectedHigh);
  }

  @Test
  public void getRangeTest() {
    Bytes firstByte = Bytes.fromHexString("0x000000000000000a");
    Bytes secondByte = Bytes.fromHexString("0x000000000000000b");
    Bytes thirdByte = Bytes.fromHexString("0x000000000000000c");
    Bytes fourthByte = Bytes.fromHexString("0x000000000000000d");
    Bytes32 bytes32 = Bytes32.wrap(Bytes.concatenate(firstByte, secondByte, thirdByte, fourthByte));

    BaseTheta baseTheta = BaseTheta.fromBytes32(bytes32);

    assertThat(baseTheta.getRange(3, 6, 2)).isEqualTo(Bytes.fromHexString("0x000a"));
    assertThat(baseTheta.getRange(1, 5, 3)).isEqualTo(Bytes.fromHexString("0x00000c"));
    assertThat(baseTheta.getRange(1, 0, 1)).isEqualTo(Bytes.fromHexString("0x00"));
  }

  @Test
  public void getTest() {
    Bytes firstByte = Bytes.fromHexString("0x000000000000000a"); // baseTheta[3]
    Bytes secondByte = Bytes.fromHexString("0x000000000000000b"); // baseTheta[2]
    Bytes thirdByte = Bytes.fromHexString("0x000000000000000c"); // baseTheta[1]
    Bytes fourthByte = Bytes.fromHexString("0x000000000000000d"); // baseTheta[0]
    Bytes32 bytes32 = Bytes32.wrap(Bytes.concatenate(firstByte, secondByte, thirdByte, fourthByte));

    BaseTheta baseTheta = BaseTheta.fromBytes32(bytes32);

    assertThat(baseTheta.get(0, 7)).isEqualTo(Bytes.fromHexString("0x0d").get(0)); // single byte
    assertThat(baseTheta.get(2, 7)).isEqualTo(Bytes.fromHexString("0x0b").get(0)); // single byte
  }

  @Test
  public void setTest() {
    Bytes firstByte = Bytes.fromHexString("0x000000000000000a");
    Bytes secondByte = Bytes.fromHexString("0x000000000000000b");
    Bytes thirdByte = Bytes.fromHexString("0x000000000000000c");
    Bytes fourthByte = Bytes.fromHexString("0x000000000000000d");
    Bytes32 bytes32 = Bytes32.wrap(Bytes.concatenate(firstByte, secondByte, thirdByte, fourthByte));

    BaseTheta baseTheta = BaseTheta.fromBytes32(bytes32);

    // equal before
    baseTheta.set(0, 7, fourthByte.get(0)); // 00
    baseTheta.set(3, 0, fourthByte.get(7)); // 0d

    assertThat(baseTheta.get(0)).isEqualTo(Bytes.fromHexString("0x0000000000000000"));
    assertThat(baseTheta.get(3)).isEqualTo(Bytes.fromHexString("0x0d0000000000000a"));

    // equal after modifications
    assertThat(baseTheta.get(0, 7)).isZero();
    assertThat(baseTheta.get(2, 7)).isEqualTo(Bytes.fromHexString("0x0b").get(0));
    assertThat(baseTheta.get(3, 0)).isEqualTo(Bytes.fromHexString("0x0d").get(0));
  }

  @Test
  public void setBytesTest() {
    BaseTheta aBaseTheta = BaseTheta.fromBytes32(UInt256.valueOf(43532));

    Bytes a0 = Bytes.fromHexString("0x000000000000aa0c");
    assertThat(aBaseTheta.get(0)).isEqualTo(a0);
    assertThat(aBaseTheta.get(1).isZero()).isTrue();
    assertThat(aBaseTheta.get(2).isZero()).isTrue();
    assertThat(aBaseTheta.get(3).isZero()).isTrue();

    Bytes b3 = Bytes.fromHexString("0x533a124790000000");
    Bytes b2 = Bytes.fromHexString("0xfaa47d49bf1d1e67");
    Bytes b1 = Bytes.fromHexString("0x952951f4425bf6f3");
    Bytes b0 = Bytes.fromHexString("0x0000000000d55835");
    Bytes32 bytes32 = Bytes32.wrap(Bytes.concatenate(b3, b2, b1, b0));
    BaseTheta bBaseTheta = BaseTheta.fromBytes32(bytes32);

    assertThat(bBaseTheta.get(3)).isEqualTo(b3);
    assertThat(bBaseTheta.get(2)).isEqualTo(b2);
    assertThat(bBaseTheta.get(1)).isEqualTo(b1);
    assertThat(bBaseTheta.get(0)).isEqualTo(b0);

    bBaseTheta.setChunk(0, b3);
    assertThat(bBaseTheta.get(0)).isEqualTo(b3);
  }
}
