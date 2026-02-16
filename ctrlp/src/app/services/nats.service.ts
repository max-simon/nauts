import { Injectable } from '@angular/core';
import { BehaviorSubject } from 'rxjs';
import {
  connect,
  NatsConnection,
  tokenAuthenticator,
  usernamePasswordAuthenticator,
  nkeyAuthenticator,
  credsAuthenticator,
} from 'nats.ws';

export type ConnectionStatus = 'disconnected' | 'connecting' | 'connected' | 'reconnecting' | 'error';

export type AuthMethod =
  | { type: 'none' }
  | { type: 'token'; token: string }
  | { type: 'userpass'; username: string; password: string }
  | { type: 'nkey'; seed: string }
  | { type: 'creds'; contents: string };

export interface ConnectParams {
  url: string;
  bucket: string;
  auth: AuthMethod;
}

const SESSION_KEY = 'nauts-connect-params';

@Injectable({ providedIn: 'root' })
export class NatsService {
  private connection: NatsConnection | null = null;
  private kvBucket: unknown = null;
  private connectPromise: Promise<void> | null = null;
  private currentParams: ConnectParams | null = null;

  readonly connectionStatus$ = new BehaviorSubject<ConnectionStatus>('disconnected');

  getSavedParams(): ConnectParams | null {
    try {
      const raw = sessionStorage.getItem(SESSION_KEY);
      return raw ? JSON.parse(raw) : null;
    } catch {
      return null;
    }
  }

  async connect(params: ConnectParams): Promise<void> {
    if (this.connection) {
      return;
    }

    // Deduplicate concurrent connect calls
    if (this.connectPromise) {
      return this.connectPromise;
    }

    this.currentParams = params;
    this.connectPromise = this.doConnect(params);
    try {
      await this.connectPromise;
      sessionStorage.setItem(SESSION_KEY, JSON.stringify(params));
    } finally {
      this.connectPromise = null;
    }
  }

  async reconnect(): Promise<void> {
    if (!this.currentParams) {
      throw new Error('No previous connection params');
    }
    await this.disconnect();
    await this.connect(this.currentParams);
  }

  private async doConnect(params: ConnectParams): Promise<void> {
    this.connectionStatus$.next('connecting');

    try {
      const opts: Record<string, unknown> = {
        servers: params.url,
        timeout: 5000,
        maxReconnectAttempts: 10,
        reconnectTimeWait: 2000,
      };

      const encoder = new TextEncoder();

      switch (params.auth.type) {
        case 'nkey':
          opts['authenticator'] = nkeyAuthenticator(encoder.encode(params.auth.seed));
          break;
        case 'creds':
          opts['authenticator'] = credsAuthenticator(encoder.encode(params.auth.contents));
          break;
      }

      this.connection = await connect(opts);
      this.connectionStatus$.next('connected');

      // Monitor connection status in the background
      this.monitorConnection();
    } catch (err) {
      this.connectionStatus$.next('error');
      throw err;
    }
  }

  private monitorConnection(): void {
    if (!this.connection) return;
    const conn = this.connection;
    (async () => {
      for await (const s of conn.status()) {
        switch (s.type) {
          case 'reconnecting':
            this.connectionStatus$.next('reconnecting');
            break;
          case 'reconnect':
            this.connectionStatus$.next('connected');
            this.kvBucket = null;
            break;
          case 'disconnect':
            this.connectionStatus$.next('disconnected');
            break;
        }
      }
    })();
  }

  async checkBucketExists(): Promise<boolean> {
    if (!this.connection || !this.currentParams) {
      throw new Error('NATS not connected');
    }
    try {
      const js = this.connection.jetstream();
      const kv = await js.views.kv(this.currentParams.bucket, { bindOnly: true });
      // Actually try to access the bucket to verify it exists
      await kv.status();
      return true;
    } catch (err: unknown) {
      // Only treat 404 (stream not found) as "bucket missing".
      // Re-throw everything else (e.g. 503 JetStream not enabled).
      if (isBucketNotFound(err)) {
        return false;
      }
      throw err;
    }
  }

  async createBucket(): Promise<void> {
    if (!this.connection || !this.currentParams) {
      throw new Error('NATS not connected');
    }
    const js = this.connection.jetstream();
    this.kvBucket = await js.views.kv(this.currentParams.bucket);
  }

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  async getKvBucket(): Promise<any> {
    if (this.kvBucket) {
      return this.kvBucket;
    }

    if (!this.connection || !this.currentParams) {
      throw new Error('NATS not connected');
    }

    const js = this.connection.jetstream();
    this.kvBucket = await js.views.kv(this.currentParams.bucket, { bindOnly: true });
    return this.kvBucket;
  }

  getConnection(): any {
    if (!this.connection) {
      throw new Error('NATS not connected');
    }
    return this.connection;
  }

  async disconnect(): Promise<void> {
    const conn = this.connection;
    this.connection = null;
    this.kvBucket = null;
    sessionStorage.removeItem(SESSION_KEY);
    if (conn) {
      try {
        await conn.drain();
      } catch {
        try { conn.close(); } catch { /* already closed */ }
      }
      this.connectionStatus$.next('disconnected');
    }
  }
}

interface NatsError extends Error {
  code: string | number;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  api_error?: { code: number | string; description: string; err_code?: number };
}

function isNatsError(err: unknown): err is NatsError {
  return err instanceof Error && 'code' in err;
}

function isBucketNotFound(err: unknown): boolean {
  if (!isNatsError(err)) {
    return false;
  }

  // Check both top-level code and api_error code for 404
  // Handle both string '404' and numeric 404
  if (err.code === 404 || err.code === '404') {
    return true;
  }
  if (err.api_error?.code === 404 || err.api_error?.code === '404') {
    return true;
  }
  if (err.api_error?.err_code === 404 || err.api_error?.err_code === 10059) {
    return true;
  }
  // Check description as last resort
  if (err.api_error?.description?.toLowerCase().includes('stream not found')) {
    return true;
  }
  return false;
}

export function describeNatsError(err: unknown): string {
  if (isNatsError(err)) {
    switch (err.code) {
      case '503':
        return 'Could not reach JetStream. Ensure JetStream is enabled on the server and the credentials have permission to access it.';
      case '401':
        return 'Authentication failed. Check your credentials.';
      case '403':
        return 'Authorization denied. The current user lacks the required permissions.';
      case '408':
        return 'Request timed out. The NATS server did not respond in time.';
    }
    if (err.api_error?.description) {
      return err.api_error.description;
    }
  }
  if (err instanceof Error) {
    return err.message;
  }
  return 'An unknown error occurred';
}
