package linea.rlp

import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.apache.tuweni.bytes.Bytes
import org.hyperledger.besu.datatypes.Hash
import org.hyperledger.besu.ethereum.core.Block
import org.hyperledger.besu.ethereum.core.BlockBody
import org.hyperledger.besu.ethereum.core.BlockHeader
import org.hyperledger.besu.ethereum.core.BlockHeaderFunctions
import org.hyperledger.besu.ethereum.core.ParsedExtraData
import org.hyperledger.besu.ethereum.core.Transaction
import org.hyperledger.besu.ethereum.core.Withdrawal
import org.hyperledger.besu.ethereum.rlp.RLPInput

object BesuRlpBlobDecoder : BesuBlockRlpDecoder {
  val log: Logger = LogManager.getLogger(BesuRlpBlobDecoder::class.java)
  val transactionDecoder: NoSignatureTransactionDecoder = NoSignatureTransactionDecoder()

  // 1.Decompressor places Block's hash in parentHash
  // Because we are reusing Geth/Besu rlp encoding that recalculate the hashes.
  // so here we override the hash function to use the parentHash as the hash
  // 2. we don't compresse extraData, so just returning null
  val hashFunction: BlockHeaderFunctions = object : BlockHeaderFunctions {
    override fun hash(blockHeader: BlockHeader): Hash = blockHeader.parentHash
    override fun parseExtraData(blockHeader: BlockHeader): ParsedExtraData? = null
  }

  override fun decode(block: ByteArray): Block {
    log.trace("Decoding block from RLP blob: rawRlpSize={}", block.size)
    return decode(org.hyperledger.besu.ethereum.rlp.RLP.input(Bytes.wrap(block)), hashFunction)
  }

  fun decode(rlpInput: RLPInput, hashFunction: BlockHeaderFunctions): Block {
    rlpInput.enterList()

    // Read the header
    val header: BlockHeader = BlockHeader.readFrom(rlpInput, hashFunction)

    // Use NoSignatureTransactionDecoder to decode transactions
    val transactions: List<Transaction> = rlpInput.readList(transactionDecoder::decode)

    // Read the ommers
    val ommers: List<BlockHeader> = rlpInput.readList { rlp: RLPInput ->
      BlockHeader.readFrom(rlp, hashFunction)
    }

    // Read the withdrawals
    if (!rlpInput.isEndOfCurrentList) {
      rlpInput.readList<Any>(Withdrawal::readFrom)
    }

    rlpInput.leaveList()
    return Block(header, BlockBody(transactions, ommers))
  }
}
