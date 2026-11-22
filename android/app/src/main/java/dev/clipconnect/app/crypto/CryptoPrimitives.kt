package dev.clipconnect.app.crypto

import java.nio.ByteBuffer
import java.nio.ByteOrder
import javax.crypto.Cipher
import javax.crypto.Mac
import javax.crypto.spec.GCMParameterSpec
import javax.crypto.spec.SecretKeySpec

/**
 * Low-level crypto primitives for [Crypto]: HMAC-SHA256, HKDF-expand, and the
 * 12-byte zero-prefix counter nonce layout. Kept separate so [Crypto] reads as
 * a clean public API.
 */
private const val HKDF_BLOCK_LEN = 32 // SHA-256 block size in bytes

internal fun Crypto.hmacSha256(key: ByteArray, data: ByteArray): ByteArray {
    val mac = Mac.getInstance("HmacSHA256")
    val spec = SecretKeySpec(key, "HmacSHA256")
    mac.init(spec)
    return mac.doFinal(data)
}

internal fun Crypto.expand(prk: ByteArray, info: String, keyLen: Int): ByteArray {
    // HKDF-Expand
    val n = (keyLen + 31) / 32 // number of blocks
    val okm = ByteArray(keyLen)
    var block = ByteArray(0)
    for (i in 0 until n) {
        val input = block + info.toByteArray(Charsets.UTF_8) + byteArrayOf((i + 1).toByte())
        block = hmacSha256(prk, input)
        System.arraycopy(block, 0, okm, i * 32, minOf(32, keyLen - i * 32))
    }
    return okm
}

/**
 * Build 12-byte nonce: 4 zero bytes + 8-byte big-endian counter.
 * Must match laptop's nonce layout: [0x00, 0x00, 0x00, 0x00, counter(8 bytes)]
 */
internal fun Crypto.buildNonce(counter: Long): ByteArray {
    return ByteBuffer.allocate(12).apply {
        order(ByteOrder.BIG_ENDIAN)
        position(4)
        putLong(counter)
    }.array()
}
