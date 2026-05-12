package net.consensys.zkevm.load.model

import net.consensys.zkevm.load.model.inner.Request
import java.math.BigInteger

class ExecutionDetails(val request: Request) {
  private lateinit var txToBlock: List<Map<Wallet, List<TransactionDetailResult>>>

  fun addContractDeployment(contractCreationTx: TransactionDetail) {
    contractCreationTxs.add(contractCreationTx)
  }

  fun addInitialization(initializeWallets: Map<Wallet, List<TransactionDetail>>) {
    initializationRequests.add(initializeWallets)
  }

  fun addTestTxs(newTxs: Map<Wallet, List<TransactionDetail>>) {
    walletsToTransactions.add(newTxs)
  }

  fun mapToBlocks(txHashToBlockId: Map<String, BigInteger>) {
    txToBlock = walletsToTransactions.map { step ->
      step.mapValues { v -> v.value.map { tx -> TransactionDetailResult(tx, txHashToBlockId.get(tx.hash)) } }
    }
  }

  private val contractCreationTxs: ArrayList<TransactionDetail> = ArrayList()
  private val initializationRequests: ArrayList<Map<Wallet, List<TransactionDetail>>> = ArrayList()

  @Transient val walletsToTransactions: ArrayList<Map<Wallet, List<TransactionDetail>>> = ArrayList()
}

class TransactionDetailResult(val tx: TransactionDetail, val blockId: BigInteger?)
