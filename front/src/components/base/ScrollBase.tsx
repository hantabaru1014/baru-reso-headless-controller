export default function ScrollBase({
  height,
  children,
}: {
  height: string;
  children: React.ReactNode;
}) {
  return (
    <div className="relative" style={{ height }}>
      <div className="absolute inset-0 overflow-y-auto">{children}</div>
    </div>
  );
}
