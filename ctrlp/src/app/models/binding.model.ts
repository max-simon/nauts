export interface Binding {
  role: string;
  account: string;
  policies: string[];
}

export interface BindingEntry {
  binding: Binding;
  revision: number;
}
