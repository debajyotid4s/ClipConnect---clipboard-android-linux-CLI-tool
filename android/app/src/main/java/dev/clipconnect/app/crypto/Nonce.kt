package dev.clipconnect.app.crypto

import java.nio.ByteBuffer
import java.nio.ByteOrder

/**
 * Build 12-byte nonce: 4 zero bytes + 8-byte big-endian counter.
 * Must match laptop's nonce layout: [0x00, 0x00, 0x00, 0x00, counter(8 bytes)]
 */
internal fun buildNonce(counter: Long): ByteArray {
    return ByteBuffer.allocate(12).apply {
        order(ByteOrder.BIG_ENDIAN)
        position(4)
        putLong(counter)
    }.array()
}
