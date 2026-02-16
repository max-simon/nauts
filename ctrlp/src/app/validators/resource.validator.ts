/**
 * Resource validator for NAUTS Resource Names (NRN).
 * Based on the resource validation logic from policy/resource.go
 */

export interface ResourceValidationError {
  resource: string;
  message: string;
}

interface ParsedResource {
  type: string;
  identifier: string;
  subIdentifier?: string;
}

/**
 * Validates a NAUTS resource name (NRN).
 * Returns null if valid, or an error message if invalid.
 */
export function validateResource(resource: string): string | null {
  if (!resource || resource.trim() === '') {
    return 'Empty resource';
  }

  const parsed = parseResource(resource);
  if (!parsed) {
    return 'Invalid resource format. Expected: <type>:<identifier>[:<sub-identifier>]';
  }

  const { type, identifier, subIdentifier } = parsed;

  // Validate type
  if (!['nats', 'js', 'kv'].includes(type)) {
    return `Unknown resource type: ${type}`;
  }

  // Validate identifiers are not empty
  if (!identifier) {
    return 'Empty identifier';
  }

  if (subIdentifier === '') {
    return 'Empty sub-identifier';
  }

  // Type-specific validation
  switch (type) {
    case 'nats':
      return validateNATSResource(identifier, subIdentifier);
    case 'js':
      return validateJSResource(identifier, subIdentifier);
    case 'kv':
      return validateKVResource(identifier, subIdentifier);
    default:
      return `Unknown resource type: ${type}`;
  }
}

function parseResource(resource: string): ParsedResource | null {
  const parts = resource.split(':');
  
  if (parts.length < 2) {
    return null;
  }

  if (parts.length === 2) {
    return {
      type: parts[0],
      identifier: parts[1],
    };
  }

  if (parts.length === 3) {
    return {
      type: parts[0],
      identifier: parts[1],
      subIdentifier: parts[2],
    };
  }

  // More than 3 parts - invalid
  return null;
}

/**
 * Validates NATS subject resources.
 * Rules:
 * - Subject: both * and > wildcards allowed
 * - Queue: only * wildcard allowed (no >)
 */
function validateNATSResource(subject: string, queue?: string): string | null {
  // Subject can have * and >
  const subjectError = validateWildcards(subject, true, true);
  if (subjectError) {
    return `Invalid subject: ${subjectError}`;
  }

  // Queue can only have *
  if (queue) {
    const queueError = validateWildcards(queue, true, false);
    if (queueError) {
      return `Invalid queue: ${queueError}`;
    }
  }

  return null;
}

/**
 * Validates JetStream stream/consumer resources.
 * Rules:
 * - Stream: only * wildcard allowed (no >)
 * - Consumer: only * wildcard allowed (no >)
 */
function validateJSResource(stream: string, consumer?: string): string | null {
  // Stream can only have *
  const streamError = validateWildcards(stream, true, false);
  if (streamError) {
    return `Invalid stream: ${streamError}`;
  }

  // Consumer can only have *
  if (consumer) {
    const consumerError = validateWildcards(consumer, true, false);
    if (consumerError) {
      return `Invalid consumer: ${consumerError}`;
    }
  }

  return null;
}

/**
 * Validates KV bucket/key resources.
 * Rules:
 * - Bucket: only * wildcard allowed (no >)
 * - Key: both * and > wildcards allowed
 */
function validateKVResource(bucket: string, key?: string): string | null {
  // Bucket can only have *
  const bucketError = validateWildcards(bucket, true, false);
  if (bucketError) {
    return `Invalid bucket: ${bucketError}`;
  }

  // Key can have * and >
  if (key) {
    const keyError = validateWildcards(key, true, true);
    if (keyError) {
      return `Invalid key: ${keyError}`;
    }
  }

  return null;
}

/**
 * Validates wildcards in a value.
 * @param value - The value to validate
 * @param allowStar - Whether * wildcard is allowed
 * @param allowGT - Whether > wildcard is allowed
 */
function validateWildcards(value: string, allowStar: boolean, allowGT: boolean): string | null {
  // Skip validation for template variables - they will be validated after interpolation
  if (value.includes('{{') && value.includes('}}')) {
    return null;
  }

  if (!allowStar && value.includes('*')) {
    return '* wildcard not allowed';
  }

  if (!allowGT && value.includes('>')) {
    return '> wildcard not allowed';
  }

  // Validate > placement - must be at the end of a token
  if (value.includes('>')) {
    const gtError = validateGTPlacement(value);
    if (gtError) {
      return gtError;
    }
  }

  return null;
}

/**
 * Ensures > is only used as a terminal wildcard.
 * Valid: "foo.>" or ">"
 * Invalid: ">.foo" or "foo.>.bar"
 */
function validateGTPlacement(value: string): string | null {
  const tokens = value.split('.');
  
  for (let i = 0; i < tokens.length; i++) {
    const token = tokens[i];
    
    if (token === '>') {
      // > must be the last token
      if (i !== tokens.length - 1) {
        return '> wildcard must be the last token';
      }
    } else if (token.includes('>')) {
      // > must be the entire token, not part of it
      return '> wildcard must be the entire token';
    }
  }

  return null;
}

/**
 * Validates all resources in a policy and returns a map of resource -> error message.
 */
export function validatePolicyResources(policy: { statements: Array<{ resources: string[] }> }): Map<string, string> {
  const errors = new Map<string, string>();
  
  for (const statement of policy.statements) {
    for (const resource of statement.resources) {
      const error = validateResource(resource);
      if (error) {
        errors.set(resource, error);
      }
    }
  }
  
  return errors;
}
