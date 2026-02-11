#!/bin/bash

# Check if a profile was provided
if [ -z "$1" ]; then
  echo "Usage: ./extract_aws_debug.sh <aws-profile> [region]"
  exit 1
fi

PROFILE=$1
REGION=${2:-eu-central-1}

# Run the command and capture stderr (2) into stdout (1)
# We use 'sts get-caller-identity' as a lightweight way to trigger a request
DEBUG_OUTPUT=$(aws sts get-caller-identity --profile "$PROFILE" --region "$REGION" --debug 2>&1)
# Filter for the specific debug line
LOG_LINE=$(echo "$DEBUG_OUTPUT" | grep "DEBUG - Sending http request:")

if [ -z "$LOG_LINE" ]; then
  echo "Error: Could not find the HTTP request debug line."
  exit 1
fi

# Extract using sed
DATE=$(echo "$LOG_LINE" | sed -n "s/.*'X-Amz-Date': b'\([^']*\)'.*/\1/p")
TOKEN=$(echo "$LOG_LINE" | sed -n "s/.*'X-Amz-Security-Token': b'\([^']*\)'.*/\1/p")
AUTH=$(echo "$LOG_LINE" | sed -n "s/.*'Authorization': b'\([^']*\)'.*/\1/p")

# Using printf to ensure valid quoting and structure
printf '{"date":"%s","authorization":"%s","securityToken":"%s","region":"%s"}' \
  "$DATE" "$AUTH" "$TOKEN" "$REGION"