package build.linea.tuweni

import linea.kotlin.toULong
import org.apache.tuweni.bytes.Bytes32
import java.math.BigInteger

fun ByteArray.toBytes32(): Bytes32 = Bytes32.wrap(this)
fun ByteArray.sliceAsBytes32(sliceIndex: Int): Bytes32 = Bytes32.wrap(this, /*offset*/sliceIndex * 32)
fun Bytes32.toULong(): ULong = BigInteger(this.toArray()).toULong()
