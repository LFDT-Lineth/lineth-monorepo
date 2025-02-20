package linea.domain

import kotlinx.datetime.Clock
import kotlinx.datetime.Instant
import linea.kotlin.ByteArrayExt

val zeroHash = ByteArray(32) { 0 }
val zeroAddress = ByteArray(20) { 0 }

fun createBlock(
  number: ULong = 0UL,
  hash: ByteArray = ByteArrayExt.random32(),
  gasLimit: ULong = 60_000_000UL,
  gasUsed: ULong = 30_000_000UL,
  difficulty: ULong = 2UL,
  parentHash: ByteArray = ByteArrayExt.random32(),
  stateRoot: ByteArray = ByteArrayExt.random32(),
  receiptsRoot: ByteArray = ByteArrayExt.random32(),
  logsBloom: ByteArray = ByteArrayExt.random(size = 256),
  ommersHash: ByteArray = ByteArrayExt.random32(),
  timestamp: Instant = Clock.System.now(),
  extraData: ByteArray = ByteArrayExt.random32(),
  baseFeePerGas: ULong = 7UL,
  transactionsRoot: ByteArray = ByteArrayExt.random32(),
  transactions: List<Transaction> = emptyList()
): Block {
  return Block(
    number = number,
    hash = hash,
    parentHash = parentHash,
    ommersHash = ommersHash,
    miner = zeroAddress,
    stateRoot = stateRoot,
    transactionsRoot = transactionsRoot,
    receiptsRoot = receiptsRoot,
    logsBloom = logsBloom,
    difficulty = difficulty,
    gasLimit = gasLimit,
    gasUsed = gasUsed,
    timestamp = timestamp.epochSeconds.toULong(),
    extraData = extraData,
    mixHash = zeroHash,
    nonce = 0UL,
    baseFeePerGas = baseFeePerGas,
    transactions = transactions,
    ommers = emptyList()
  )
}

/**
 * This is very similar to Block class,
 * but creating DTO to avoid coupling with domain model,
 * some fields are not present in domain model, e.g uncles
 *
 * This is meant to help creating fake JSON-RPC server
 */
class EthGetBlockResponseDTO(
  val number: ULong,
  val hash: ByteArray,
  val parentHash: ByteArray,
  val miner: ByteArray,
  val stateRoot: ByteArray,
  val transactionsRoot: ByteArray,
  val receiptsRoot: ByteArray,
  val logsBloom: ByteArray,
  val difficulty: ULong,
  val gasLimit: ULong,
  val gasUsed: ULong,
  val timestamp: ULong,
  val extraData: ByteArray,
  val mixHash: ByteArray,
  val nonce: ULong,
  val baseFeePerGas: ULong?,
  val sha3Uncles: ByteArray, // ommersHash
  val size: ULong,
  val totalDifficulty: ULong,
  val transactions: List<ByteArray>,
  val uncles: List<ByteArray> = emptyList()
)

fun Block?.toEthGetBlockResponse(
  size: ULong = 10UL * 1024UL,
  totalDifficulty: ULong = this?.difficulty ?: 0UL
): EthGetBlockResponseDTO? {
  if (this == null) return null
  return EthGetBlockResponseDTO(
    number = this.number,
    hash = this.hash,
    parentHash = this.parentHash,
    miner = this.miner,
    stateRoot = this.stateRoot,
    transactionsRoot = this.transactionsRoot,
    receiptsRoot = this.receiptsRoot,
    logsBloom = this.logsBloom,
    difficulty = this.difficulty,
    gasLimit = this.gasLimit,
    gasUsed = this.gasUsed,
    timestamp = this.timestamp,
    extraData = this.extraData,
    mixHash = this.mixHash,
    nonce = this.nonce,
    baseFeePerGas = this.baseFeePerGas,
    sha3Uncles = this.ommersHash,
    size = size,
    totalDifficulty = totalDifficulty,
    transactions = emptyList<ByteArray>()
  )
}
