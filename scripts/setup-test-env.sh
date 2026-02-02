#!/bin/bash
#
# Setup a test environment for nauts
#
# This script creates:
# - An nsc operator and accounts for the EntityProvider
# - A users.json file for the UserIdentityProvider
# - policies.json and groups.json for the NautsProvider
#
# Usage: ./scripts/setup-test-env.sh [output-dir]
#
# The output directory defaults to ./testenv

set -e

# Configuration
OUTPUT_DIR="${1:-./testenv}"
OPERATOR_NAME="test-operator"
ACCOUNT_NAME="test-account"

# Users to create (username:password:groups)
# Groups are comma-separated
USERS=(
    "alice:alice123:admin,workers"
    "bob:bob456:workers"
    "charlie:charlie789:readonly"
)

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
    exit 1
}

# Check for required tools
check_requirements() {
    info "Checking requirements..."

    if ! command -v nsc &> /dev/null; then
        error "nsc is not installed. Install it from: https://github.com/nats-io/nsc"
    fi

    if ! command -v go &> /dev/null; then
        error "go is not installed. Required for generating bcrypt hashes."
    fi

    if ! command -v nkeys &> /dev/null; then
        warn "nkeys CLI not found. Will use nsc for key operations."
    fi
}

# Generate bcrypt hash using Go
generate_bcrypt_hash() {
    local password="$1"
    local tmpdir
    tmpdir=$(mktemp -d)
    trap "rm -rf '$tmpdir'" RETURN

    cat > "$tmpdir/bcrypt.go" <<'GOCODE'
package main

import (
    "fmt"
    "os"
    "golang.org/x/crypto/bcrypt"
)

func main() {
    if len(os.Args) < 2 {
        fmt.Fprintln(os.Stderr, "usage: bcrypt <password>")
        os.Exit(1)
    }
    hash, err := bcrypt.GenerateFromPassword([]byte(os.Args[1]), bcrypt.DefaultCost)
    if err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
    }
    fmt.Print(string(hash))
}
GOCODE

    (cd "$tmpdir" && go mod init bcrypt >/dev/null 2>&1 && go mod tidy >/dev/null 2>&1 && go run bcrypt.go "$password")
}

# Setup nsc environment
setup_nsc() {
    local nsc_dir="$OUTPUT_DIR/nsc"

    info "Setting up nsc environment in $nsc_dir..."

    # Set NSC environment
    export NKEYS_PATH="$nsc_dir/keys"
    export NSC_HOME="$nsc_dir"

    # Create directory structure
    mkdir -p "$nsc_dir"

    # Initialize nsc data directory
    nsc env -s "$nsc_dir/nats" > /dev/null 2>&1 || true

    # Create operator
    info "Creating operator: $OPERATOR_NAME"
    nsc add operator "$OPERATOR_NAME" --sys 2>/dev/null || {
        warn "Operator may already exist, continuing..."
    }

    # Select operator
    nsc env -o "$OPERATOR_NAME" > /dev/null 2>&1

    # Create account
    info "Creating account: $ACCOUNT_NAME"
    nsc add account "$ACCOUNT_NAME" 2>/dev/null || {
        warn "Account may already exist, continuing..."
    }

    # Get account public key for reference
    local account_pub
    account_pub=$(nsc describe account "$ACCOUNT_NAME" -J | grep -o '"sub": "[^"]*"' | head -1 | cut -d'"' -f4)
    info "Account public key: $account_pub"

    echo "$nsc_dir"
}

# Create users.json file
create_users_file() {
    local users_file="$OUTPUT_DIR/users.json"

    info "Creating users file: $users_file"

    echo '{' > "$users_file"
    echo '  "users": {' >> "$users_file"

    local first=true
    for user_entry in "${USERS[@]}"; do
        IFS=':' read -r username password groups <<< "$user_entry"

        if [ "$first" = true ]; then
            first=false
        else
            echo ',' >> "$users_file"
        fi

        info "  Creating user: $username (groups: $groups)"

        # Generate bcrypt hash
        local hash
        hash=$(generate_bcrypt_hash "$password")

        # Convert comma-separated groups to JSON array
        local groups_json
        groups_json=$(echo "$groups" | tr ',' '\n' | sed 's/^/"/;s/$/"/' | tr '\n' ',' | sed 's/,$//' | sed 's/^/[/;s/$/]/')

        cat >> "$users_file" <<EOF
    "$username": {
      "account": "$ACCOUNT_NAME",
      "groups": $groups_json,
      "passwordHash": "$hash",
      "attributes": {
        "email": "$username@example.com"
      }
    }
EOF
    done

    echo '' >> "$users_file"
    echo '  }' >> "$users_file"
    echo '}' >> "$users_file"
}

# Create policies.json file
create_policies_file() {
    local policies_file="$OUTPUT_DIR/policies.json"

    info "Creating policies file: $policies_file"

    cat > "$policies_file" <<'EOF'
[
  {
    "id": "admin-full-access",
    "name": "Admin Full Access",
    "description": "Full access to all NATS subjects",
    "statements": [
      {
        "effect": "allow",
        "actions": ["nats.pub", "nats.sub"],
        "resources": ["nats:>"]
      },
      {
        "effect": "allow",
        "actions": ["js.*"],
        "resources": ["js:*", "js:*:*"]
      },
      {
        "effect": "allow",
        "actions": ["kv.*"],
        "resources": ["kv:*", "kv:*:*"]
      }
    ]
  },
  {
    "id": "workers-access",
    "name": "Workers Access",
    "description": "Access to worker queues and user-specific subjects",
    "statements": [
      {
        "effect": "allow",
        "actions": ["nats.pub", "nats.sub"],
        "resources": ["nats:workers.>"]
      },
      {
        "effect": "allow",
        "actions": ["nats.pub", "nats.sub"],
        "resources": ["nats:user.{{ user.id }}.>"]
      },
      {
        "effect": "allow",
        "actions": ["nats.req"],
        "resources": ["nats:api.>"]
      }
    ]
  },
  {
    "id": "readonly-access",
    "name": "Read-Only Access",
    "description": "Subscribe-only access to public subjects",
    "statements": [
      {
        "effect": "allow",
        "actions": ["nats.sub"],
        "resources": ["nats:public.>"]
      }
    ]
  },
  {
    "id": "default-inbox",
    "name": "Default Inbox Access",
    "description": "Allow reply inbox for request-reply patterns",
    "statements": [
      {
        "effect": "allow",
        "actions": ["nats.sub"],
        "resources": ["nats:_INBOX.>"]
      }
    ]
  }
]
EOF
}

# Create groups.json file
create_groups_file() {
    local groups_file="$OUTPUT_DIR/groups.json"

    info "Creating groups file: $groups_file"

    cat > "$groups_file" <<'EOF'
[
  {
    "id": "default",
    "name": "Default",
    "description": "Default group for all users",
    "policies": ["default-inbox"]
  },
  {
    "id": "admin",
    "name": "Administrators",
    "description": "Full system access",
    "policies": ["admin-full-access"]
  },
  {
    "id": "workers",
    "name": "Workers",
    "description": "Worker processes with queue access",
    "policies": ["workers-access"]
  },
  {
    "id": "readonly",
    "name": "Read-Only Users",
    "description": "Users with read-only access",
    "policies": ["readonly-access"]
  }
]
EOF
}

# Generate a user nkey for testing
generate_user_nkey() {
    local keys_dir="$OUTPUT_DIR/user-keys"
    mkdir -p "$keys_dir"

    info "Generating user nkey for testing..."

    # Use nsc to generate a user key
    local user_seed
    local user_pub

    # Create a temporary user to get a key
    nsc add user temp-user -a "$ACCOUNT_NAME" 2>/dev/null || true
    user_pub=$(nsc describe user temp-user -a "$ACCOUNT_NAME" -J 2>/dev/null | grep -o '"sub": "[^"]*"' | head -1 | cut -d'"' -f4)

    # Find the seed file
    local seed_file
    seed_file=$(find "$OUTPUT_DIR/nsc/keys" -name "${user_pub}.nk" 2>/dev/null | head -1)

    if [ -n "$seed_file" ] && [ -f "$seed_file" ]; then
        cp "$seed_file" "$keys_dir/user.nk"
        echo "$user_pub" > "$keys_dir/user.pub"
        info "User public key: $user_pub"
        info "User seed saved to: $keys_dir/user.nk"
    else
        warn "Could not find user seed file. You may need to generate keys manually."
    fi

    # Clean up temp user
    nsc delete user temp-user -a "$ACCOUNT_NAME" 2>/dev/null || true
}

# Print usage instructions
print_usage() {
    local nsc_dir="$OUTPUT_DIR/nsc"
    local user_pub_file="$OUTPUT_DIR/user-keys/user.pub"
    local user_pub=""

    if [ -f "$user_pub_file" ]; then
        user_pub=$(cat "$user_pub_file")
    else
        user_pub="<USER_PUBLIC_KEY>"
    fi

    echo ""
    echo "=========================================="
    echo "Test environment created successfully!"
    echo "=========================================="
    echo ""
    echo "Directory structure:"
    echo "  $OUTPUT_DIR/"
    echo "  ├── nsc/              # nsc data directory"
    echo "  ├── user-keys/        # Test user keys"
    echo "  ├── users.json        # User credentials"
    echo "  ├── policies.json     # Access policies"
    echo "  └── groups.json       # User groups"
    echo ""
    echo "Users created:"
    for user_entry in "${USERS[@]}"; do
        IFS=':' read -r username password groups <<< "$user_entry"
        echo "  - $username (password: $password, groups: $groups)"
    done
    echo ""
    echo "To build and run nauts:"
    echo ""
    echo "  # Build the CLI"
    echo "  go build -o bin/nauts ./cmd/nauts"
    echo ""
    echo "  # Authenticate as alice (ephemeral key)"
    echo "  ./bin/nauts \\"
    echo "    -nsc-dir $nsc_dir \\"
    echo "    -operator $OPERATOR_NAME \\"
    echo "    -policies $OUTPUT_DIR/policies.json \\"
    echo "    -groups $OUTPUT_DIR/groups.json \\"
    echo "    -users $OUTPUT_DIR/users.json \\"
    echo "    -username alice \\"
    echo "    -password alice123"
    echo ""
    echo "  # Or with a specific user public key"
    echo "  ./bin/nauts \\"
    echo "    -nsc-dir $nsc_dir \\"
    echo "    -operator $OPERATOR_NAME \\"
    echo "    -policies $OUTPUT_DIR/policies.json \\"
    echo "    -groups $OUTPUT_DIR/groups.json \\"
    echo "    -users $OUTPUT_DIR/users.json \\"
    echo "    -username bob \\"
    echo "    -password bob456 \\"
    echo "    -user-pubkey $user_pub"
    echo ""
}

# Main
main() {
    echo "========================================"
    echo "nauts Test Environment Setup"
    echo "========================================"
    echo ""

    check_requirements

    # Create output directory
    mkdir -p "$OUTPUT_DIR"
    OUTPUT_DIR=$(cd "$OUTPUT_DIR" && pwd)  # Get absolute path

    info "Output directory: $OUTPUT_DIR"

    # Setup components
    setup_nsc
    create_policies_file
    create_groups_file
    create_users_file
    generate_user_nkey

    print_usage
}

main "$@"
