import { Policy, Statement } from '../models/policy.model';

export interface ValidationError {
  field: string;
  message: string;
}

export function validatePolicy(policy: Partial<Policy>): ValidationError[] {
  const errors: ValidationError[] = [];

  if (!policy.name?.trim()) {
    errors.push({ field: 'name', message: 'Name is required' });
  }

  if (!policy.account?.trim()) {
    errors.push({ field: 'account', message: 'Account is required' });
  }

  if (!policy.statements || policy.statements.length === 0) {
    errors.push({ field: 'statements', message: 'At least one statement is required' });
  } else {
    policy.statements.forEach((stmt, i) => {
      errors.push(...validateStatement(stmt, i));
    });
  }

  return errors;
}

export function validateStatement(stmt: Partial<Statement>, index: number): ValidationError[] {
  const errors: ValidationError[] = [];
  const prefix = `statements.${index}`;

  if (stmt.effect !== 'allow') {
    errors.push({ field: `${prefix}.effect`, message: 'Effect must be "allow"' });
  }

  if (!stmt.actions || stmt.actions.length === 0) {
    errors.push({ field: `${prefix}.actions`, message: 'At least one action is required' });
  }

  if (!stmt.resources || stmt.resources.length === 0) {
    errors.push({ field: `${prefix}.resources`, message: 'At least one resource is required' });
  }

  return errors;
}
