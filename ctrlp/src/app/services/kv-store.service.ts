import { Injectable } from '@angular/core';
import { NatsService } from './nats.service';

export interface KvEntry<T = unknown> {
  key: string;
  value: T;
  revision: number;
}

export class ConflictError extends Error {
  constructor(key: string) {
    super(`Conflict updating key: ${key}`);
    this.name = 'ConflictError';
  }
}

export class NotFoundError extends Error {
  constructor(key: string) {
    super(`Key not found: ${key}`);
    this.name = 'NotFoundError';
  }
}

@Injectable({ providedIn: 'root' })
export class KvStoreService {
  constructor(private nats: NatsService) {}

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  private decode<T>(entry: any): KvEntry<T> {
    const value = JSON.parse(new TextDecoder().decode(entry.value)) as T;
    return {
      key: entry.key,
      value,
      revision: entry.revision,
    };
  }

  async get<T>(key: string): Promise<KvEntry<T> | null> {
    const kv = await this.nats.getKvBucket();
    try {
      const entry = await kv.get(key);
      if (!entry || entry.operation !== 'PUT') {
        return null;
      }
      return this.decode<T>(entry);
    } catch {
      return null;
    }
  }

  async list<T>(prefix: string): Promise<KvEntry<T>[]> {
    return this.listByFilter<T>(prefix + '>');
  }

  async listByFilter<T>(filter: string): Promise<KvEntry<T>[]> {
    const kv = await this.nats.getKvBucket();
    const entries: KvEntry<T>[] = [];
    const keys = await kv.keys(filter);

    for await (const key of keys) {
      const entry = await this.get<T>(key);
      if (entry) {
        entries.push(entry);
      }
    }

    return entries;
  }

  async listAll<T>(): Promise<KvEntry<T>[]> {
    const kv = await this.nats.getKvBucket();
    const entries: KvEntry<T>[] = [];
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    let watcher: any;
    let earlyInit = false;
    watcher = await kv.watch({
      initializedFn: () => {
        if (watcher) {
          watcher.stop();
        } else {
          earlyInit = true;
        }
      },
    });
    if (earlyInit) {
      watcher.stop();
      return entries;
    }
    for await (const entry of watcher) {
      if (entry.operation === 'PUT') {
        entries.push(this.decode<T>(entry));
      }
    }
    return entries;
  }

  async put<T>(key: string, value: T, revision?: number): Promise<number> {
    const kv = await this.nats.getKvBucket();
    try {
      const encoded = new TextEncoder().encode(JSON.stringify(value));
      if (revision !== undefined) {
        return await kv.put(key, encoded, { previousSeq: revision });
      }
      return await kv.put(key, encoded);
    } catch (err: unknown) {
      if (err instanceof Error && err.message.includes('wrong last sequence')) {
        throw new ConflictError(key);
      }
      throw err;
    }
  }

  async create<T>(key: string, value: T): Promise<number> {
    const kv = await this.nats.getKvBucket();
    try {
      return await kv.create(key, new TextEncoder().encode(JSON.stringify(value)));
    } catch (err: unknown) {
      if (err instanceof Error && err.message.includes('wrong last sequence')) {
        throw new ConflictError(key);
      }
      throw err;
    }
  }

  async delete(key: string, revision: number): Promise<void> {
    const kv = await this.nats.getKvBucket();
    try {
      await kv.delete(key, { previousSeq: revision });
    } catch (err: unknown) {
      if (err instanceof Error && err.message.includes('wrong last sequence')) {
        throw new ConflictError(key);
      }
      throw err;
    }
  }

  async watch<T>(callback: (entry: KvEntry<T> | null, operation: 'PUT' | 'DEL') => void): Promise<() => void> {
    const kv = await this.nats.getKvBucket();
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const watcher: any = await kv.watch({ ignoreDeletes: false, includeHistory: false });
    
    (async () => {
      try {
        for await (const entry of watcher) {
          if (entry.operation === 'PUT') {
            callback(this.decode<T>(entry), 'PUT');
          } else if (entry.operation === 'DEL' || entry.operation === 'PURGE') {
            callback({ key: entry.key, value: null as T, revision: entry.revision }, 'DEL');
          }
        }
      } catch (err) {
        console.error('Watch error:', err);
      }
    })();

    return () => watcher.stop();
  }
}
