package lineth.rlp

import lineth.domain.BinaryDecoder
import lineth.domain.BinaryDecoderAsync
import lineth.domain.BinaryEncoder
import lineth.domain.BinaryEncoderAsync
import org.hyperledger.besu.ethereum.core.Block

interface BesuBlockRlpEncoder : BinaryEncoder<Block>
interface BesuBlockRlpEncoderAsync : BinaryEncoderAsync<Block>
interface BesuBlockRlpDecoder : BinaryDecoder<Block>
interface BesuBlockRlpDecoderAsync : BinaryDecoderAsync<Block>
