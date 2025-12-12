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
package net.consensys.linea.zktracer.instructionprocessing.selfdestructTests;

import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.ToyAccount;
import net.consensys.linea.zktracer.instructionprocessing.utilities.*;
import org.hyperledger.besu.crypto.KeyPair;
import org.hyperledger.besu.crypto.SECP256K1;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.datatypes.Wei;
import org.junit.jupiter.api.extension.ExtendWith;

@ExtendWith(UnitTestWatcher.class)
public class SeveralSelfDestructsInARowModifyingStorageTests extends TracerTestBase {
  Address modifyStorageThenSelfDestructAddress = Address.fromHexString("ffc0de");
  Hash hash = Hash.fromHexString("modifyStorageThenSelfDestruct");
  Address multipleCallsAddress = Address.fromHexString("ca11e7");

  public static KeyPair keyPair = new SECP256K1().generateKeyPair();
  public static Address userAddress =
      Address.extract(Hash.hash(keyPair.getPublicKey().getEncodedBytes()));
  public static ToyAccount userAccount =
      ToyAccount.builder().balance(Wei.fromEth(10)).nonce(99).address(userAddress).build();

  private ToyAccount modifyStorageThenSelfDestruct =
      ToyAccount.builder()
          .balance(Wei.fromEth(1))
          .nonce(13)
          .address(modifyStorageThenSelfDestructAddress)
          .code(
              SelfDestructs.storageTouchingSelfDestructorRewardsZeroAddress(chainConfig).compile())
          .build();
}
