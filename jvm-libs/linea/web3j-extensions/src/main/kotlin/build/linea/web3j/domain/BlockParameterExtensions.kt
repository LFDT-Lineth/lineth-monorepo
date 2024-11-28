package build.linea.web3j.domain

import net.consensys.linea.BlockParameter
import net.consensys.toBigInteger
import org.web3j.protocol.core.DefaultBlockParameter

fun BlockParameter.toWeb3j(): DefaultBlockParameter {
  return when (this) {
    is BlockParameter.Tag -> DefaultBlockParameter.valueOf(this.getTag())
    is BlockParameter.BlockNumber -> DefaultBlockParameter.valueOf(this.getNumber().toBigInteger())
  }
}
