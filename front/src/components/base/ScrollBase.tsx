export default function ScrollBase({
  height,
  children,
}: {
  height: string;
  children: React.ReactNode;
}) {
  return (
    <div
      style={{
        position: "relative",
        height,
      }}
    >
      <div
        style={{
          position: "absolute",
          top: 0,
          right: 0,
          bottom: 0,
          left: 0,
          overflowY: "scroll",
        }}
      >
        {children}
      </div>
    </div>
  );
}
