const GLOBAL_PREFIX = '_global';
const GLOBAL_POLICY_PREFIX = '_global:';

// Global policy prefix utilities
export function stripGlobalPrefix(policyId: string): string {
  return policyId.startsWith(GLOBAL_POLICY_PREFIX)
    ? policyId.substring(GLOBAL_POLICY_PREFIX.length)
    : policyId;
}

export function addGlobalPrefix(policyId: string): string {
  return `${GLOBAL_POLICY_PREFIX}${policyId}`;
}

export function hasGlobalPrefix(policyId: string): boolean {
  return policyId.startsWith(GLOBAL_POLICY_PREFIX);
}

export function policyKey(account: string, id: string): string {
  const prefix = account === '*' ? GLOBAL_PREFIX : account;
  return `${prefix}.policy.${id}`;
}

export function bindingKey(account: string, role: string): string {
  if(account === "*") {
    throw new Error("Bindings can not be global");
  }
  return `${account}.binding.${role}`;
}

export function policyPrefix(account: string): string {
  const prefix = account === '*' ? GLOBAL_PREFIX : account;
  return `${prefix}.policy.`;
}

export function bindingPrefix(account: string): string {
  if(account === "*") {
    throw new Error("Bindings can not be global");
  }
  return `${account}.binding.`;
}

export function parsePolicyKey(key: string): { account: string; id: string } | null {
  const match = key.match(/^(.+)\.policy\.(.+)$/);
  if (!match) return null;
  return {
    account: match[1] === GLOBAL_PREFIX ? '*' : match[1],
    id: match[2],
  };
}

export function parseBindingKey(key: string): { account: string; role: string } | null {
  const match = key.match(/^(.+)\.binding\.(.+)$/);
  if (!match) return null;
  return {
    account: match[1],
    role: match[2],
  };
}

export function accountFromKeyPrefix(prefix: string): string {
  return prefix === GLOBAL_PREFIX ? '*' : prefix;
}

export function isGlobalAccount(account: string): boolean {
  return account === '*';
}
