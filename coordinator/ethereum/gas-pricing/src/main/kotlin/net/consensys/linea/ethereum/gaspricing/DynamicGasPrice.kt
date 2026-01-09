package net.consensys.linea.ethereum.gaspricing

import linea.domain.FeeHistory
import linea.kotlin.decodeHex
import linea.kotlin.encodeHex
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.nio.ByteBuffer

interface FeesFetcher {
  fun getL1EthGasPriceData(): SafeFuture<FeeHistory>
}

interface L2CalldataSizeAccumulator {
  fun getSumOfL2CalldataSize(blockNumber: ULong): SafeFuture<ULong>
}

interface HistoricVariableCostProvider {
  fun getVariableCost(blockNumber: ULong): SafeFuture<Double>
}

fun interface FeesCalculator {
  fun calculateFees(feeHistory: FeeHistory): Double
}

interface MinerExtraDataCalculator {
  fun calculateMinerExtraData(feeHistory: FeeHistory): MinerExtraDataV1
}

interface GasPriceUpdater {
  fun updateMinerGasPrice(gasPrice: ULong): SafeFuture<Unit>
}

enum class MinerExtraDataVersions(val version: Byte) {
  V1(0x1),
}

data class MinerExtraDataV1(
  val fixedCostInKWei: UInt,
  val variableCostInKWei: UInt,
  val ethGasPriceInKWei: UInt,
) {
  val version: Byte = MinerExtraDataVersions.V1.version

  fun encode(): String {
    val byteBuffer = ByteBuffer.allocate(32)
    byteBuffer.put(version)
    byteBuffer.putInt(fixedCostInKWei.toInt())
    byteBuffer.putInt(variableCostInKWei.toInt())
    byteBuffer.putInt(ethGasPriceInKWei.toInt())
    return byteBuffer.array().encodeHex()
  }

  companion object {
    fun decodeV1(minerExtraData: String): MinerExtraDataV1 {
      val byteBuffer = ByteBuffer.wrap(minerExtraData.decodeHex())
      val version = byteBuffer.get()
      require(version == 0x1.toByte()) {
        "version=$version is not supported! " +
          "One of: ${MinerExtraDataVersions.values()} is expected"
      }
      val fixedCostInKWei = byteBuffer.getInt().toUInt()
      val variableCostInKWei = byteBuffer.getInt().toUInt()
      val ethGasPriceInKWei = byteBuffer.getInt().toUInt()
      return MinerExtraDataV1(fixedCostInKWei, variableCostInKWei, ethGasPriceInKWei)
    }
  }

  override fun toString(): String {
    return "MinerExtraData(version=$version" +
      " fixedCostInKWei=$fixedCostInKWei KWei" +
      " variableCostInKWei=$variableCostInKWei KWei" +
      " ethGasPriceInKWei=$ethGasPriceInKWei Kwei)"
  }
}

interface ExtraDataUpdater {
  fun updateMinerExtraData(extraData: MinerExtraDataV1): SafeFuture<Unit>
}
