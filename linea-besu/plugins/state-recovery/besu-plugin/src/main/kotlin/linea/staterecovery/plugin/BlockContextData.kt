package linea.staterecovery.plugin

import org.hyperledger.besu.plugin.data.BlockBody
import org.hyperledger.besu.plugin.data.BlockContext
import org.hyperledger.besu.plugin.data.BlockHeader

data class BlockContextData(
  private val blockHeader: BlockHeader,
  private val blockBody: BlockBody,
) : BlockContext {
  override fun getBlockHeader(): BlockHeader = blockHeader

  override fun getBlockBody(): BlockBody = blockBody
}
