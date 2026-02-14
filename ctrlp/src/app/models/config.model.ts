export interface AppConfig {
  nats: {
    url: string;
    bucket: string;
    credentials?: string;
    nkey?: string;
  };
  ui: {
    defaultAccount: string;
    showGlobalPolicies: boolean;
  };
}
