package net.consensys.linea.zktracer.instructionprocessing.callTests;

import static net.consensys.linea.zktracer.Utils.*;

import java.util.List;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.ToyAccount;
import net.consensys.linea.testing.ToyExecutionEnvironmentV2;
import net.consensys.linea.testing.ToyTransaction;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.crypto.KeyPair;
import org.hyperledger.besu.crypto.SECP256K1;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.junit.jupiter.api.Disabled;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;

/*

CALL DELEGATION TEST

We test code delegation to different types of targets
  (1) TARGET is a Smart Contract
       CALL
     -------->  - EOA
                   |---------------> Smart Contract
                     (delegated to)
  (2) TARGET is an EOA delegated to another EOA
       CALL
     -------->  - EOA1
                   |---------------> EOA2
                     (delegated to)
  (3) TARGET is an EOA delegated to itself
       CALL
     -------->  - EOA  <---------|
                   |-------------|
                    (delegated to)
  (4) TARGET is an EOA delegated to another EOA delegated to a Smart Contract
       CALL
     -------->  - EOA1
                   |---------------> EOA2
                     (delegated to)   |---------------> Smart Contract
                                        (delegated to)
      Note : to avoid a loop of delegations, exec client retrieve only the first code and then stop following the delegation chain.
      Hence, the Smart Contract is never reached and its code never executed
  (5) TARGET is a precompile
       CALL
     -------->  - EOA
                   |---------------> PRC, here P256_VERIFY
                     (delegated to)
 */

// TODO: reenable once 7702 feature is merged
@Disabled
public class CallDelegation extends TracerTestBase {

  /* Smart contract byte code
      ADDRESS
      SELFBALANCE
      CODESIZE
      CALLER
      CALLVALUE
      GAS
  */

  final String smcBytecode = "0x30473833345a";
  final ToyAccount smcAccount =
      ToyAccount.builder()
          .address(Address.fromHexString("1234"))
          .nonce(90)
          .code(Bytes.concatenate(Bytes.fromHexString(smcBytecode)))
          .build();
  final String delegationCodeToSmc = addDelegationPrefixToAccount(smcAccount);

  // Sender account setting
  final KeyPair keyPair = new SECP256K1().generateKeyPair();
  final Address senderAddress =
      Address.extract(Hash.hash(keyPair.getPublicKey().getEncodedBytes()));
  final ToyAccount senderAccount =
      ToyAccount.builder()
          .balance(Wei.of(1_550_000_000_000L))
          .nonce(23)
          .address(senderAddress)
          .build();

  /*
  (1) TARGET is a Smart Contract
       CALL
     -------->  - EOA
                   |---------------> Smart Contract
                     (delegated to)
   */
  @Test
  void EOADelegatedToSmc(TestInfo testInfo) {

    ToyAccount eoaDelegatedToSmc =
        ToyAccount.builder()
            .address(Address.fromHexString("ca11ee")) // identity caller
            .nonce(99)
            .code(Bytes.concatenate(Bytes.fromHexString(delegationCodeToSmc)))
            .build();

    Transaction tx =
        ToyTransaction.builder()
            .sender(senderAccount)
            .keyPair(keyPair)
            .to(eoaDelegatedToSmc)
            .build();

    ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
        .accounts(List.of(senderAccount, smcAccount, eoaDelegatedToSmc))
        .transaction(tx)
        .build()
        .run();
  }

  /*
  (2) TARGET is an EOA delegated to another EOA
       CALL
     -------->  - EOA1
                   |---------------> EOA2
                     (delegated to)
   */
  @Test
  void EOA1DelegatedEOA2(TestInfo testInfo) {

    ToyAccount eoa2 =
        ToyAccount.builder()
            .address(Address.fromHexString("ca11ee2")) // identity caller
            .nonce(80)
            .build();

    String delegationCodeToEoa2 = addDelegationPrefixToAccount(eoa2);

    ToyAccount eoa1DelegatedToEoa2 =
        ToyAccount.builder()
            .address(Address.fromHexString("ca11ee1")) // identity caller
            .nonce(99)
            .code(Bytes.concatenate(Bytes.fromHexString(delegationCodeToEoa2)))
            .build();

    Transaction tx =
        ToyTransaction.builder()
            .sender(senderAccount)
            .keyPair(keyPair)
            .to(eoa1DelegatedToEoa2)
            .build();

    ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
        .accounts(List.of(senderAccount, eoa1DelegatedToEoa2, eoa2))
        .transaction(tx)
        .build()
        .run();
  }

  /*
  (3) TARGET is an EOA delegated to itself
       CALL
     -------->  - EOA  <---------|
                   |-------------|
                    (delegated to)
   */
  @Test
  void EOADelegatedToItself(TestInfo testInfo) {

    Address eoaAddress = Address.fromHexString("ca11ee1");

    String delegationCodeToEoa = addDelegationPrefixToAddress(eoaAddress);

    ToyAccount eoa =
        ToyAccount.builder()
            .address(eoaAddress) // identity caller
            .nonce(99)
            .code(Bytes.concatenate(Bytes.fromHexString(delegationCodeToEoa)))
            .build();

    Transaction tx =
        ToyTransaction.builder().sender(senderAccount).keyPair(keyPair).to(eoa).build();

    ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
        .accounts(List.of(senderAccount, eoa))
        .transaction(tx)
        .build()
        .run();
  }

  /*
  (4) TARGET is an EOA delegated to another EOA
       CALL
     -------->  - EOA1
                   |---------------> EOA2
                     (delegated to)   |---------------> Smart Contract
                                        (delegated to)
   */
  @Test
  void EOA1DelegatedEOA2DelegatedToSmc(TestInfo testInfo) {

    ToyAccount eoa2DelegatedToSmc =
        ToyAccount.builder()
            .address(Address.fromHexString("ca11ee2")) // identity caller
            .nonce(80)
            .code(Bytes.concatenate(Bytes.fromHexString(delegationCodeToSmc)))
            .build();

    String delegationCodeToEoa2 = addDelegationPrefixToAccount(eoa2DelegatedToSmc);

    ToyAccount eoa1DelegatedToEoa2 =
        ToyAccount.builder()
            .address(Address.fromHexString("ca11ee1")) // identity caller
            .nonce(99)
            .code(Bytes.concatenate(Bytes.fromHexString(delegationCodeToEoa2)))
            .build();

    Transaction tx =
        ToyTransaction.builder()
            .sender(senderAccount)
            .keyPair(keyPair)
            .to(eoa1DelegatedToEoa2)
            .build();

    ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
        .accounts(List.of(senderAccount, smcAccount, eoa1DelegatedToEoa2, eoa2DelegatedToSmc))
        .transaction(tx)
        .build()
        .run();
  }

  /*
   (5) TARGET is a precompile
      CALL
    -------->  - EOA
                  |---------------> PRC, here P256_VERIFY
                    (delegated to)
  */
  @Test
  void targetIsPrc(TestInfo testInfo) {

    String delegationCodeToP256Verify = addDelegationPrefixToAddress(Address.P256_VERIFY);
    ToyAccount eoaDelegatedToPrc =
        ToyAccount.builder()
            .address(Address.fromHexString("ca11ee")) // identity caller
            .nonce(99)
            .code(Bytes.concatenate(Bytes.fromHexString(delegationCodeToP256Verify)))
            .build();

    Transaction tx =
        ToyTransaction.builder()
            .sender(senderAccount)
            .keyPair(keyPair)
            .to(eoaDelegatedToPrc)
            .build();

    ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
        .accounts(List.of(senderAccount, eoaDelegatedToPrc))
        .transaction(tx)
        .build()
        .run();
  }
}
