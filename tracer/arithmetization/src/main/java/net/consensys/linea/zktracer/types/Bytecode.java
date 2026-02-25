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

package net.consensys.linea.zktracer.types;

import static net.consensys.linea.zktracer.Trace.EOA_DELEGATED_CODE_LENGTH;
import static net.consensys.linea.zktracer.module.hub.AccountSnapshot.EIP_7702_DELEGATION_INDICATOR_BYTES;

import java.util.Objects;
import java.util.Optional;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.logging.Logger;
import lombok.Getter;
import lombok.experimental.Accessors;
import lombok.extern.slf4j.Slf4j;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.evm.Code;

@Slf4j
@Accessors(fluent = true)
/** This class is intended to store a bytecode and its memoized hash. */
public final class Bytecode {
  /** initializing the executor service before creating the EMPTY bytecode. */
  private static final ExecutorService executorService = Executors.newCachedThreadPool();

  /** The empty bytecode. */
  public static Bytecode EMPTY = new Bytecode(Bytes.EMPTY);

  /** The bytecode. */
  @Getter private final Bytes bytecode;

  /** The bytecode hash; is null by default and computed & memoized on the fly when required. */
  private Hash hash;

  /**
   * Create an instance from {@link Bytes}.
   *
   * @param bytes the bytecode
   */
  public Bytecode(Bytes bytes) {
    this.bytecode = Objects.requireNonNullElse(bytes, Bytes.EMPTY);
  }

  /**
   * Create an instance from Besu {@link Code}.
   *
   * @param code the bytecode
   */
  public Bytecode(Code code) {
    this.bytecode = code.getBytes();
    this.hash = code.getCodeHash();
  }

  /**
   * Get the size of the bytecode, in bytes.
   *
   * @return the bytecode size
   */
  public int getSize() {
    return this.bytecode.size();
  }

  /**
   * Returns whether the bytecode is empty or not.
   *
   * @return true if the bytecode is empty
   */
  public boolean isEmpty() {
    return this.bytecode.isEmpty();
  }

  /**
   * Compute the bytecode hash if required, then return it.
   *
   * @return the bytecode hash
   */
  public Hash getCodeHash() {
    if (this.hash == null) {
      if (this.bytecode.isEmpty()) {
        this.hash = Hash.EMPTY;
      } else {
        this.hash = Hash.hash(this.bytecode);
      }
    }
    return this.hash;
  }

  /**
   * {@link #isDelegated()} returns true if the byte code <b>looks like</b> it is account delegation
   * code i.e. is of the form
   *
   * <p><div style="text-align:center"><b>0x ef 01 00 + <20 bytes></b></div>
   *
   * <p><b>Note.</b> One must be careful when using this method that byte code may <b>seem delegated
   * without actually being delegated</b>, e.g. if the byte code corresponds to initialization code.
   */
  public boolean isDelegated() {
    return isDelegated(this.bytecode);
  }

  public static boolean isDelegated(Bytes code) {
    if (code.size() != EOA_DELEGATED_CODE_LENGTH) {
      return false;
    }
    return code.slice(0, 3).equals(EIP_7702_DELEGATION_INDICATOR_BYTES);
  }

  /**
   * {@link #getDelegateAddress()} produces the delegation address of a delegated account, or
   * returns the empty optional if the account isn't delegated.
   *
   * <p>It also logs the following unconventional event: the account is delegated to the ZERO
   * address i.e. its byte code is
   *
   * <p><div style="text-align:center"><b>0x ef 01 00 + Address.ZERO</b></div>
   *
   * <p><b>Note.</b> The state can conceivably contain an account with this bytecode. Such an
   * account could be inserted into the state via the genesis block or through a historic deployment
   * predating the <b>0xEF</b> prefix restriction EIP.
   *
   * <p>TODO: are there any such tests in the EVM test-suite ?
   */
  public Optional<Address> getDelegateAddress() {
    if (!isDelegated()) {
      return Optional.empty();
    }

    final Address delegateAddress = Address.wrap(bytecode.slice(3, Address.SIZE));

    if (delegateAddress.equals(Address.ZERO)) {
      Logger logger = Logger.getLogger(Bytecode.class.getName());
      logger.info("[INFO] Bytecode of the form 0x ef 01 00 <ZERO address>");
    }

    return Optional.of(delegateAddress);
  }
}
