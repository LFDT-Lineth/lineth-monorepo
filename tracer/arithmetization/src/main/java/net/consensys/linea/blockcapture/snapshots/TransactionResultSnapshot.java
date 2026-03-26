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
package net.consensys.linea.blockcapture.snapshots;

import java.util.List;
import java.util.Set;
import net.consensys.linea.zktracer.ConflationAwareOperationTracer;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.units.bigints.UInt256;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Log;
import org.hyperledger.besu.datatypes.Transaction;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.evm.account.Account;
import org.hyperledger.besu.evm.worldstate.WorldView;

/**
 * Records details regarding the outcome of a given transaction, such as whether it succeeded or
 * failed.
 *
 * @param status true if the transaction was successful, false otherwise
 * @param output the bytes output from the transaction (i.e. return data).
 * @param logs the logs emitted by this transaction
 * @param gasUsed the gas used by the entire transaction
 * @param accounts accounts touched by this transaction.
 * @param storage storage locations touched by this transaction.
 * @param selfDestructs accounts which self-destructed during this transaction
 */
public record TransactionResultSnapshot(
    boolean status,
    String output,
    List<String> logs,
    long gasUsed,
    List<AccountSnapshot> accounts,
    List<StorageSnapshot> storage,
    List<String> selfDestructs) {

  /**
   * Construct a suitable checker for this result snapshot which can be used within a transaction
   * processor.
   *
   * @return An implementation of OperationTracer which checks the outcome of this transaction
   *     matches.
   */
  public ConflationAwareOperationTracer check() {
    return new TransactionChecker();
  }

  /** Simple wrapper which checks the transaction result matches an expected outcome. */
  private class TransactionChecker implements ConflationAwareOperationTracer {
    /**
     * Used to indicate that the result checker already failed this transaction and, hence, not to
     * do it again!
     */
    private String failure = null;

    @Override
    public void traceEndTransaction(
        WorldView world,
        Transaction tx,
        boolean status,
        Bytes output,
        List<Log> logs,
        long gasUsed,
        Set<Address> selfDestructs,
        long timeNs) {
      // Check whether we already failed this transaction (or not).
      if (failure != null) {
        // Force complete failure.
        throw new RuntimeException(failure);
      } else {
        String hash = tx.getHash().getBytes().toHexString();
        boolean expStatus = TransactionResultSnapshot.this.status();
        // Check against expected result
        if (TransactionResultSnapshot.this.status() != status) {
          failTransaction(
              "tx "
                  + hash
                  + " outcome does not match expected outcome (expected "
                  + expStatus
                  + ", was "
                  + status
                  + ")");
        }
        long expGas = TransactionResultSnapshot.this.gasUsed();
        if (expGas != gasUsed) {
          failTransaction(
              "tx "
                  + hash
                  + " gas used does not match expected gas used  (expected "
                  + expGas
                  + ", was "
                  + gasUsed
                  + ")");
        }
        if (!TransactionResultSnapshot.this.output().equals(output.toHexString())) {
          failTransaction("tx " + hash + " output does not match expected output");
        }
        // Convert logs into hex strings
        List<String> actualLogStrings = logs.stream().map(l -> l.getData().toHexString()).toList();
        if (!actualLogStrings.equals(TransactionResultSnapshot.this.logs())) {
          failTransaction("tx " + hash + " logs do not match expected logs");
        }
        // Check each account
        for (AccountSnapshot expAccount : TransactionResultSnapshot.this.accounts()) {
          Address address = Address.fromHexString(expAccount.address());
          Account actAccount = world.get(address);
          // Check balance
          Wei expBalance = Wei.fromHexString(expAccount.balance());
          Wei actBalance = actAccount.getBalance();
          if (!expBalance.equals(actBalance)) {
            failTransaction(
                "tx "
                    + hash
                    + " balance of account "
                    + address
                    + " (0x"
                    + actBalance.toHexString()
                    + ") does not match expected value (0x"
                    + expBalance.toHexString()
                    + ")");
          }
          // Check nonce
          long expNonce = expAccount.nonce();
          long actNonce = actAccount.getNonce();
          if (expNonce != actNonce) {
            failTransaction(
                "tx "
                    + hash
                    + " nonce of account "
                    + address
                    + " ("
                    + actNonce
                    + ") does not match expected value ("
                    + expNonce
                    + ")");
          }
          // Check code
          Bytes expCode = Bytes.fromHexString(expAccount.code());
          Bytes actCode = actAccount.getCode();
          if (!expCode.equals(actCode)) {
            failTransaction(
                "tx "
                    + hash
                    + " code of account "
                    + address
                    + " ("
                    + actCode
                    + ") does not match expected value ("
                    + expCode
                    + ")");
          }
        }
        // Check each storage location
        for (StorageSnapshot expStorage : TransactionResultSnapshot.this.storage()) {
          Address address = Address.fromHexString(expStorage.address());
          UInt256 key = UInt256.fromHexString(expStorage.key());
          UInt256 expValue = UInt256.fromHexString(expStorage.value());
          Account actAccount = world.get(address);
          UInt256 actValue = actAccount.getStorageValue(key);
          if (!actValue.equals(expValue)) {
            failTransaction(
                "tx "
                    + hash
                    + " storage at "
                    + address
                    + ":"
                    + key
                    + "("
                    + actValue
                    + ") does not match expected value ("
                    + expValue
                    + ")");
          }
        }
      }
    }

    @Override
    public void traceStartConflation(long numBlocksInConflation) {
      throw new UnsupportedOperationException();
    }

    @Override
    public void traceEndConflation(WorldView state) {
      throw new UnsupportedOperationException();
    }

    private void failTransaction(String msg) {
      // Mark transaction as failed in order to prevent the true failing reason from being hidden .
      // This is necessary because MainnetTransactionProcessor re-executes
      // <code>traceEndTransaction()</code>
      // upon receiving a <code>RuntimeException</code>.
      this.failure = msg;
      //
      throw new RuntimeException(msg);
    }
  }
}
