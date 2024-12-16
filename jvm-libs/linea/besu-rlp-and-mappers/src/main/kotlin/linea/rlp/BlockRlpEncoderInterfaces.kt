package linea.rlp

import linea.domain.BinaryDecoder
import linea.domain.BinaryDecoderAsync
import linea.domain.BinaryEncoder
import linea.domain.BinaryEncoderAsync
import org.hyperledger.besu.ethereum.core.Block

interface BesuBlockRlpEncoder : BinaryEncoder<Block>
interface BesuBlockRlpEncoderAsync : BinaryEncoderAsync<Block>
interface BesuBlockRlpDecoder : BinaryDecoder<Block>
interface BesuBlockRlpDecoderAsync : BinaryDecoderAsync<Block>
