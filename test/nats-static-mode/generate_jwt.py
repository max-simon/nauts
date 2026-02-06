"""Generate a RS256 JWT for testing.

If `rsa.key` and `rsa.pem` exist in this directory they are used.
Otherwise a new keypair is created and written out.

Usage: python generate_jwt.py [role1 role2 ...]
"""

import sys
import os
from pathlib import Path
from base64 import b64encode
import jwt
import datetime
from cryptography.hazmat.primitives.asymmetric import rsa
from cryptography.hazmat.primitives import serialization


def load_or_create_keys(dir_path: Path, priv_name: str = "rsa.key", pub_name: str = "rsa.pem"):
    priv_path = dir_path / priv_name
    pub_path = dir_path / pub_name

    if priv_path.exists() and pub_path.exists():
        private_pem = priv_path.read_bytes()
        public_pem = pub_path.read_bytes()
    else:
        private_key = rsa.generate_private_key(public_exponent=65537, key_size=2048)
        private_pem = private_key.private_bytes(
            encoding=serialization.Encoding.PEM,
            format=serialization.PrivateFormat.PKCS8,
            encryption_algorithm=serialization.NoEncryption(),
        )
        public_pem = private_key.public_key().public_bytes(
            encoding=serialization.Encoding.PEM,
            format=serialization.PublicFormat.SubjectPublicKeyInfo,
        )

        # Write files (private with restrictive perms)
        priv_path.write_bytes(private_pem)
        try:
            os.chmod(priv_path, 0o600)
        except Exception:
            pass
        pub_path.write_bytes(public_pem)

    return private_pem.decode("utf-8"), public_pem.decode("utf-8")


def main(argv):
    script_dir = Path(__file__).parent
    private_pem, public_pem = load_or_create_keys(script_dir)

    # Roles from argv, otherwise fallback to sensible defaults
    roles = argv[1:] if len(argv) > 1 else ["APP.readonly", "AUTH.full"]

    now = datetime.datetime.now(datetime.timezone.utc)
    ten_years_later = now + datetime.timedelta(days=365 * 10)

    payload = {
        "iss": "e2e",
        "iat": int(now.timestamp()),
        "exp": int(ten_years_later.timestamp()),
        "nauts": {"roles": roles},
    }

    token = jwt.encode(payload, private_pem, algorithm="RS256")

    public_key_b64 = b64encode(public_pem.encode("utf-8")).decode("utf-8")

    print("--- PUBLIC KEY (b64) ---")
    print(public_key_b64)
    print("\n--- GENERATED JWT ---")
    print(token)


if __name__ == "__main__":
    main(sys.argv)
