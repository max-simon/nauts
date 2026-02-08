#!/bin/bash
# Generate AWS SigV4 token for nauts authentication
# Usage: ./generate-aws-token.sh <profile>

set -e

PROFILE="$1"
if [ -z "$PROFILE" ]; then
    echo "Usage: $0 <profile>"
    exit 1
fi

# Set AWS config file
export AWS_CONFIG_FILE="$(dirname "$0")/aws-config"
export AWS_SHARED_CREDENTIALS_FILE="$AWS_CONFIG_FILE"

# Suppress deprecation warnings
export PYTHONWARNINGS="ignore::DeprecationWarning"

# Use Python to generate the signed request
python3 - <<'PYTHON' "$PROFILE"
import sys
import json
import os
from datetime import datetime
from urllib.request import Request, urlopen
from urllib.parse import urlencode
import hashlib
import hmac

# AWS credentials and configuration
profile = sys.argv[1]

# Read AWS config
config_file = os.environ.get('AWS_CONFIG_FILE')
config = {}
current_profile = None
with open(config_file, 'r') as f:
    for line in f:
        line = line.strip()
        if line.startswith('[profile '):
            current_profile = line[9:-1]  # Extract profile name
        elif line.startswith('['):
            current_profile = line[1:-1]
        elif '=' in line and current_profile:
            key, value = line.split('=', 1)
            if current_profile not in config:
                config[current_profile] = {}
            config[current_profile][key.strip()] = value.strip()

# Get credentials for the profile
if profile not in config:
    print(f"Profile {profile} not found", file=sys.stderr)
    sys.exit(1)

profile_config = config[profile]

# If this is a role profile, assume the role
if 'role_arn' in profile_config:
    # Use source profile credentials to assume role
    source_profile = profile_config.get('source_profile')
    if source_profile not in config:
        print(f"Source profile {source_profile} not found", file=sys.stderr)
        sys.exit(1)
    
    source_config = config[source_profile]
    access_key = source_config.get('aws_access_key_id')
    secret_key = source_config.get('aws_secret_access_key')
    region = source_config.get('region', 'us-east-1')
    role_arn = profile_config['role_arn']
    session_name = profile_config.get('role_session_name', 'nauts-test')
    
    # Assume role via STS
    service = 'sts'
    host = f'sts.{region}.amazonaws.com'
    endpoint = f'https://{host}/'
    
    # Create timestamp
    t = datetime.utcnow()
    amz_date = t.strftime('%Y%m%dT%H%M%SZ')
    date_stamp = t.strftime('%Y%m%d')
    
    # Create canonical request for AssumeRole
    method = 'POST'
    canonical_uri = '/'
    canonical_querystring = ''
    
    params = {
        'Action': 'AssumeRole',
        'RoleArn': role_arn,
        'RoleSessionName': session_name,
        'Version': '2011-06-15',
        'DurationSeconds': '3600'
    }
    
    payload = urlencode(params)
    content_type = 'application/x-www-form-urlencoded; charset=utf-8'
    
    canonical_headers = f'content-type:{content_type}\nhost:{host}\nx-amz-date:{amz_date}\n'
    signed_headers = 'content-type;host;x-amz-date'
    
    payload_hash = hashlib.sha256(payload.encode('utf-8')).hexdigest()
    canonical_request = f'{method}\n{canonical_uri}\n{canonical_querystring}\n{canonical_headers}\n{signed_headers}\n{payload_hash}'
    
    # Create string to sign
    algorithm = 'AWS4-HMAC-SHA256'
    credential_scope = f'{date_stamp}/{region}/{service}/aws4_request'
    string_to_sign = f'{algorithm}\n{amz_date}\n{credential_scope}\n{hashlib.sha256(canonical_request.encode("utf-8")).hexdigest()}'
    
    # Calculate signature
    def sign(key, msg):
        return hmac.new(key, msg.encode('utf-8'), hashlib.sha256).digest()
    
    kDate = sign(('AWS4' + secret_key).encode('utf-8'), date_stamp)
    kRegion = hmac.new(kDate, region.encode('utf-8'), hashlib.sha256).digest()
    kService = hmac.new(kRegion, service.encode('utf-8'), hashlib.sha256).digest()
    kSigning = hmac.new(kService, b'aws4_request', hashlib.sha256).digest()
    signature = hmac.new(kSigning, string_to_sign.encode('utf-8'), hashlib.sha256).hexdigest()
    
    # Create Authorization header
    authorization_header = f'{algorithm} Credential={access_key}/{credential_scope}, SignedHeaders={signed_headers}, Signature={signature}'
    
    # Make request to assume role
    req = Request(endpoint, data=payload.encode('utf-8'))
    req.add_header('Content-Type', content_type)
    req.add_header('X-Amz-Date', amz_date)
    req.add_header('Authorization', authorization_header)
    
    try:
        response = urlopen(req)
        response_data = response.read().decode('utf-8')
        
        # Parse XML response to extract temporary credentials
        # Simple XML parsing - find the values between tags
        import re
        access_key_match = re.search(r'<AccessKeyId>(.*?)</AccessKeyId>', response_data)
        secret_key_match = re.search(r'<SecretAccessKey>(.*?)</SecretAccessKey>', response_data)
        session_token_match = re.search(r'<SessionToken>(.*?)</SessionToken>', response_data)
        
        if not all([access_key_match, secret_key_match, session_token_match]):
            print(f"Failed to parse AssumeRole response", file=sys.stderr)
            sys.exit(1)
        
        access_key = access_key_match.group(1)
        secret_key = secret_key_match.group(1)
        session_token = session_token_match.group(1)
        
    except Exception as e:
        print(f"Failed to assume role: {e}", file=sys.stderr)
        sys.exit(1)
else:
    # Use direct credentials
    access_key = profile_config.get('aws_access_key_id')
    secret_key = profile_config.get('aws_secret_access_key')
    session_token = None
    region = profile_config.get('region', 'us-east-1')

# Now create GetCallerIdentity signed request
service = 'sts'
host = f'sts.{region}.amazonaws.com'
endpoint = f'https://{host}/'

# Create timestamp
t = datetime.utcnow()
amz_date = t.strftime('%Y%m%dT%H%M%SZ')
date_stamp = t.strftime('%Y%m%d')

# Create canonical request
method = 'POST'
canonical_uri = '/'
canonical_querystring = ''

params = {
    'Action': 'GetCallerIdentity',
    'Version': '2011-06-15'
}

payload = urlencode(params)
content_type = 'application/x-www-form-urlencoded; charset=utf-8'

if session_token:
    canonical_headers = f'content-type:{content_type}\nhost:{host}\nx-amz-date:{amz_date}\nx-amz-security-token:{session_token}\n'
    signed_headers = 'content-type;host;x-amz-date;x-amz-security-token'
else:
    canonical_headers = f'content-type:{content_type}\nhost:{host}\nx-amz-date:{amz_date}\n'
    signed_headers = 'content-type;host;x-amz-date'

payload_hash = hashlib.sha256(payload.encode('utf-8')).hexdigest()
canonical_request = f'{method}\n{canonical_uri}\n{canonical_querystring}\n{canonical_headers}\n{signed_headers}\n{payload_hash}'

# Create string to sign
algorithm = 'AWS4-HMAC-SHA256'
credential_scope = f'{date_stamp}/{region}/{service}/aws4_request'
string_to_sign = f'{algorithm}\n{amz_date}\n{credential_scope}\n{hashlib.sha256(canonical_request.encode("utf-8")).hexdigest()}'

# Calculate signature
def sign(key, msg):
    return hmac.new(key, msg.encode('utf-8'), hashlib.sha256).digest()

kDate = sign(('AWS4' + secret_key).encode('utf-8'), date_stamp)
kRegion = hmac.new(kDate, region.encode('utf-8'), hashlib.sha256).digest()
kService = hmac.new(kRegion, service.encode('utf-8'), hashlib.sha256).digest()
kSigning = hmac.new(kService, b'aws4_request', hashlib.sha256).digest()
signature = hmac.new(kSigning, string_to_sign.encode('utf-8'), hashlib.sha256).hexdigest()

# Create Authorization header
authorization_header = f'{algorithm} Credential={access_key}/{credential_scope}, SignedHeaders={signed_headers}, Signature={signature}'

# Output JSON token
token = {
    'authorization': authorization_header,
    'date': amz_date,
}
if session_token:
    token['securityToken'] = session_token

print(json.dumps(token))
PYTHON
