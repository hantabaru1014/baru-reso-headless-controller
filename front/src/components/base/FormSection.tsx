import { ReactNode } from "react";

/**
 * 枠線と見出しでフォームの項目をまとめるセクション.
 */
export function FormSection({
  title,
  caption,
  children,
}: {
  title: string;
  caption?: string;
  children: ReactNode;
}) {
  return (
    <section className="space-y-3 rounded-md border p-4">
      <div>
        <h3 className="text-sm font-semibold">{title}</h3>
        {caption && <p className="text-xs text-muted-foreground">{caption}</p>}
      </div>
      {children}
    </section>
  );
}
