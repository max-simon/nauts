import { Binding } from '../models/binding.model';
import { ValidationError } from './policy.validator';

export function validateBinding(binding: Partial<Binding>): ValidationError[] {
  const errors: ValidationError[] = [];

  if (!binding.role?.trim()) {
    errors.push({ field: 'role', message: 'Role is required' });
  }

  if (!binding.account?.trim()) {
    errors.push({ field: 'account', message: 'Account is required' });
  }

  if (!binding.policies || binding.policies.length === 0) {
    errors.push({ field: 'policies', message: 'At least one policy is required' });
  }

  return errors;
}
