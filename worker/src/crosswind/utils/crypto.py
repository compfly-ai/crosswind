"""Cryptographic utilities matching the Go API service."""

import base64
import hashlib

from cryptography.hazmat.primitives.ciphers.aead import AESGCM

from crosswind.config import settings


def _get_key() -> bytes:
    """Get the 32-byte AES key from the encryption key setting.

    Matches the Go implementation:
    1. Try base64 decode (strict)
    2. Try hex decode
    3. Fall back to SHA-256 hash of the key string
    """
    import re

    encoded_key = settings.encryption_key

    if not encoded_key:
        raise ValueError("ENCRYPTION_KEY not set")

    # Try base64 decode (strict validation)
    try:
        if re.match(r"^[A-Za-z0-9+/]*={0,2}$", encoded_key) and len(encoded_key) % 4 == 0:
            key = base64.b64decode(encoded_key, validate=True)
            if len(key) >= 16:
                if len(key) < 32:
                    key = hashlib.sha256(key).digest()
                elif len(key) > 32:
                    key = key[:32]
                return key
    except Exception:
        pass

    # Try hex decode
    try:
        key = bytes.fromhex(encoded_key)
        if len(key) >= 16:
            if len(key) < 32:
                key = hashlib.sha256(key).digest()
            elif len(key) > 32:
                key = key[:32]
            return key
    except Exception:
        pass

    # Fall back to SHA-256 hash
    return hashlib.sha256(encoded_key.encode()).digest()


def decrypt_credentials(encrypted: str) -> str:
    """Decrypt credentials using AES-256-GCM.

    Matches the Go Encryptor.Decrypt() implementation.

    Args:
        encrypted: The encrypted string, optionally with "encrypted:" prefix

    Returns:
        Decrypted plaintext string
    """
    if not encrypted:
        return ""

    # Remove the "encrypted:" prefix if present
    if encrypted.startswith("encrypted:"):
        encrypted = encrypted[10:]

    try:
        # Decode base64
        ciphertext = base64.b64decode(encrypted)

        # Get key
        key = _get_key()
        aesgcm = AESGCM(key)

        # Split nonce (12 bytes for GCM) and ciphertext
        nonce_size = 12
        if len(ciphertext) < nonce_size:
            raise ValueError("Ciphertext too short")

        nonce = ciphertext[:nonce_size]
        actual_ciphertext = ciphertext[nonce_size:]

        # Decrypt
        plaintext = aesgcm.decrypt(nonce, actual_ciphertext, None)
        return plaintext.decode("utf-8")

    except Exception as e:
        # If decryption fails, maybe it's not encrypted (dev mode)
        import structlog
        logger = structlog.get_logger()
        logger.warning("Failed to decrypt credentials, returning as-is", error=str(e))
        return encrypted
