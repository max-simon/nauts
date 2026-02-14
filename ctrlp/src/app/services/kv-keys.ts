const GLOBAL_PREFIX = '_global';

export function policyKey(account: string, id: string): string {
  const prefix = account === '*' ? GLOBAL_PREFIX : account;
  return `${prefix}.policy.${id}`;
}

export function bindingKey(account: string, role: string): string {
  const prefix = account === '*' ? GLOBAL_PREFIX : account;
  return `${prefix}.binding.${role}`;
}

export function globalPolicyKey(id: string): string {
  return `${GLOBAL_PREFIX}.policy.${id}`;
}

export function globalBindingKey(role: string): string {
  return `${GLOBAL_PREFIX}.binding.${role}`;
}

export function policyPrefix(account: string): string {
  const prefix = account === '*' ? GLOBAL_PREFIX : account;
  return `${prefix}.policy.`;
}

export function bindingPrefix(account: string): string {
  const prefix = account === '*' ? GLOBAL_PREFIX : account;
  return `${prefix}.binding.`;
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
    account: match[1] === GLOBAL_PREFIX ? '*' : match[1],
    role: match[2],
  };
}

export function accountFromKeyPrefix(prefix: string): string {
  return prefix === GLOBAL_PREFIX ? '*' : prefix;
}

export function isGlobalAccount(account: string): boolean {
  return account === '*';
}
