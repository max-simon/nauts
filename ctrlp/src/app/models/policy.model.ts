export interface Statement {
  effect: 'allow';
  actions: string[];
  resources: string[];
}

export interface Policy {
  id: string;
  account: string;
  name: string;
  statements: Statement[];
}

export interface PolicyEntry {
  policy: Policy;
  revision: number;
}
