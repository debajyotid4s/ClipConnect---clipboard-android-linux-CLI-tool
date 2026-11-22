package dev.clipconnect.app.ui

/**
 * Parsing helpers for manual pairing input.
 *
 * Kept pure and free of Android so the validation rules can be unit tested. The
 * UI passes raw text in; these functions return either a usable value or a
 * human-readable validation message, never an exception.
 */
object PairingParser {

    /** Result of validating raw manual-pairing input. */
    sealed interface Result {
        data class Valid(val host: String, val port: Int, val secret: ByteArray) : Result
        data class Invalid(val message: String) : Result
    }

    /** Decode a base64url secret, tolerating missing/extra padding. Returns null on any malformed input. */
    fun decodeSecret(raw: String): ByteArray? {
        val trimmed = raw.trim()
        if (trimmed.isEmpty()) return null
        // base64url -> base64 standard.
        val b64 = trimmed.replace('-', '+').replace('_', '/')
        val padded = when (b64.length % 4) {
            2 -> b64 + "=="
            3 -> b64 + "="
            else -> b64
        }
        return try {
            android.util.Base64.decode(padded, android.util.Base64.DEFAULT)
        } catch (e: Exception) {
            null
        }
    }

    /** Validate host as IPv4 or hostname using the same rule the network layer uses. */
    fun isValidHost(host: String): Boolean {
        val h = host.trim()
        if (h.isEmpty()) return false
        val parts = h.split(".")
        if (parts.size == 4) {
            for (part in parts) {
                if (part.isEmpty() || !part.all { it.isDigit() }) return false
                val num = part.toIntOrNull() ?: return false
                if (num < 0 || num > 255) return false
            }
            return true
        }
        val allNumeric = parts.all { it.isNotEmpty() && it.all { c -> c.isDigit() } }
        if (allNumeric && parts.size != 4) return false
        return h.all { it.isLetterOrDigit() || it == '.' || it == '-' }
    }

    /**
     * Validate a complete manual-pairing submission. Returns the first problem
     * found so the UI can show one inline message at a time.
     */
    fun validate(hostRaw: String, portRaw: String, secretRaw: String): Result {
        val host = hostRaw.trim()
        val portText = portRaw.trim()
        val secret = secretRaw.trim()

        if (host.isEmpty()) return Result.Invalid("Enter the laptop's IP or hostname.")
        if (!isValidHost(host)) return Result.Invalid("That doesn't look like an IP or hostname.")
        if (secret.isEmpty()) return Result.Invalid("Enter the pairing secret.")

        val port = portText.toIntOrNull()
        if (port == null) return Result.Invalid("Port must be a number.")
        if (port < 1 || port > 65535) return Result.Invalid("Port must be between 1 and 65535.")

        val decoded = decodeSecret(secret)
        if (decoded == null) return Result.Invalid("That doesn't look like a valid secret.")

        return Result.Valid(host, port, decoded)
    }

    /**
     * Parse a `clipconnect://pair?...` deep-link URL. Returns [Result.Valid] with
     * the decoded secret, or [Result.Invalid] with a message. Malformed input
     * never throws.
     */
    fun parseUri(rawValue: String): Result {
        val uri = android.net.Uri.parse(rawValue)
        if (uri.scheme != "clipconnect" || uri.host != "pair") {
            return Result.Invalid("Not a ClipConnect pairing link.")
        }
        val secretB64 = (uri.getQueryParameter("k") ?: "").trim()
        val host = (uri.getQueryParameter("h")?.split(",")?.firstOrNull() ?: "").trim()
        val port = uri.getQueryParameter("p")?.toIntOrNull() ?: 10369
        if (secretB64.isBlank() || host.isBlank()) {
            return Result.Invalid("The link is missing the host or secret.")
        }
        if (!isValidHost(host)) return Result.Invalid("That doesn't look like an IP or hostname.")
        if (port < 1 || port > 65535) return Result.Invalid("The link's port is out of range.")
        val decoded = decodeSecret(secretB64)
        if (decoded == null) return Result.Invalid("The link's secret is not valid base64.")
        return Result.Valid(host, port, decoded)
    }
}
