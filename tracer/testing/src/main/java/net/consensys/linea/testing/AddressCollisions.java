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

package net.consensys.linea.testing;

public enum AddressCollisions {
  NO_COLLISION,
  SENDER_IS_RECIPIENT,
  SENDER_IS_COINBASE,
  RECIPIENT_IS_COINBASE,
  TRIPLE_COLLISION;

  public static boolean senderRecipientCollision(final AddressCollisions collision) {
    return collision == SENDER_IS_RECIPIENT || collision == TRIPLE_COLLISION;
  }

  public static boolean senderCoinbaseCollision(final AddressCollisions collision) {
    return collision == SENDER_IS_COINBASE || collision == TRIPLE_COLLISION;
  }

  public static boolean recipientCoinbaseCollision(final AddressCollisions collision) {
    return collision == RECIPIENT_IS_COINBASE || collision == TRIPLE_COLLISION;
  }
}
