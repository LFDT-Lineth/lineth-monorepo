package linea.staterecovery.plugin

import linea.kotlin.hasSequentialElements
import net.consensys.encodeHex
import net.consensys.minusCoercingUnderflow
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.apache.tuweni.bytes.Bytes32
import org.hyperledger.besu.datatypes.Hash
import org.hyperledger.besu.evm.blockhash.BlockHashLookup
import org.hyperledger.besu.evm.frame.MessageFrame
import java.util.concurrent.ConcurrentHashMap

class BlockHashLookupWithRecoverySupport(
  val lookbackWindow: ULong,
  private val log: Logger = LogManager.getLogger(BlockHashLookupWithRecoverySupport::class.java)
) : BlockHashLookup {
  private val lookbackHashesMap = ConcurrentHashMap<ULong, ByteArray>()

  fun addLookbackHashes(blocksHashes: Map<ULong, ByteArray>) {
    require(blocksHashes.keys.toList().sorted().hasSequentialElements()) {
      "Block numbers must be sequential"
    }

    log.debug("adding hashes: {}", {
      blocksHashes.toList()
        .sortedBy { it.first }
        .map { (k, v) -> "$k=${v.encodeHex()}" }
    })

    lookbackHashesMap.putAll(blocksHashes)
  }

  fun addHeadBlockHash(blockNumber: ULong, blockHash: ByteArray) {
    lookbackHashesMap[blockNumber] = blockHash
    pruneLookBackHashes(blockNumber)
  }

  private fun pruneLookBackHashes(headBlockNumber: ULong) {
    if (headBlockNumber <= lookbackWindow) return

    lookbackHashesMap.keys.removeIf {
      // <= would wrongly remove block 0 while headBlockNumber < lookBackWindow,
      // we keep 1 block more that we need, but it's fine
      it < headBlockNumber.minusCoercingUnderflow(lookbackWindow)
    }
  }

  fun getHash(blockNumber: Long): Hash {
    val resolvedHash = lookbackHashesMap[blockNumber.toULong()]
      ?.let { Hash.wrap(Bytes32.wrap(it)) }
      ?: Hash.ZERO

    log.debug("block={} lookback hash={}", blockNumber, resolvedHash)
    return resolvedHash
  }

  override fun apply(t: MessageFrame?, blockNumber: Long): Hash {
    return getHash(blockNumber)
  }
}
