export interface FieldFooterProps {
  error?: string;
}

export function FieldFooter({ error }: FieldFooterProps) {
  return <>{error && <p className="text-sm text-destructive">{error}</p>}</>;
}
