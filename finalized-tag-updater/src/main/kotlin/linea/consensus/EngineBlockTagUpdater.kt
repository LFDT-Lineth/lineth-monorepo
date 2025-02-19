package linea.consensus

interface EngineBlockTagUpdater {
  fun lineaUpdateFinalizedBlockV1(finalizedBlockNumber: Long)
}
