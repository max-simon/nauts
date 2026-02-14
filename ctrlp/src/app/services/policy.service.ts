import { Injectable } from '@angular/core';
import { Policy, PolicyEntry } from '../models/policy.model';
import { KvStoreService } from './kv-store.service';
import { policyKey, policyPrefix, parsePolicyKey } from './kv-keys';

@Injectable({ providedIn: 'root' })
export class PolicyService {
  constructor(private kv: KvStoreService) {}

  async listPolicies(account: string): Promise<PolicyEntry[]> {
    const entries = await this.kv.list<Policy>(policyPrefix(account));
    return entries.map(e => ({ policy: e.value, revision: e.revision }));
  }

  async listGlobalPolicies(): Promise<PolicyEntry[]> {
    return this.listPolicies('*');
  }

  async listAllPolicies(): Promise<PolicyEntry[]> {
    const entries = await this.kv.listAll<Policy>();
    return entries
      .filter(e => parsePolicyKey(e.key) !== null)
      .map(e => ({ policy: e.value, revision: e.revision }));
  }

  async getPolicy(account: string, id: string): Promise<PolicyEntry | null> {
    const entry = await this.kv.get<Policy>(policyKey(account, id));
    if (!entry) return null;
    return { policy: entry.value, revision: entry.revision };
  }

  async createPolicy(policy: Policy): Promise<number> {
    policy.id = crypto.randomUUID();
    const key = policyKey(policy.account, policy.id);
    return this.kv.create(key, policy);
  }

  async updatePolicy(account: string, id: string, policy: Policy, revision: number): Promise<number> {
    const key = policyKey(account, id);
    return this.kv.put(key, policy, revision);
  }

  async deletePolicy(account: string, id: string, revision: number): Promise<void> {
    const key = policyKey(account, id);
    await this.kv.delete(key, revision);
  }
}
