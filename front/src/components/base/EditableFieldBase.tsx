import { Button, Skeleton } from "../ui";
import { Edit, Check, X } from "lucide-react";

export default function EditableFieldBase({
  editing,
  onEditStart,
  onSave,
  onCancel,
  readonly,
  isLoading,
  children,
}: {
  editing: boolean;
  onEditStart?: () => void;
  onSave?: () => void;
  onCancel?: () => void;
  readonly?: boolean;
  isLoading?: boolean;
  children: React.ReactNode;
}) {
  return (
    <div className="flex items-start gap-2">
      {isLoading ? (
        <Skeleton className="h-9 w-full" />
      ) : (
        <>
          <div className="flex-1">{children}</div>
          {!readonly &&
            (editing ? (
              <div className="flex gap-1">
                <Button
                  size="icon"
                  variant="ghost"
                  onClick={onSave}
                  aria-label="保存"
                >
                  <Check className="h-4 w-4" />
                </Button>
                <Button
                  size="icon"
                  variant="ghost"
                  onClick={onCancel}
                  aria-label="キャンセル"
                >
                  <X className="h-4 w-4" />
                </Button>
              </div>
            ) : (
              <Button
                size="icon"
                variant="ghost"
                onClick={onEditStart}
                aria-label="編集"
              >
                <Edit className="h-4 w-4" />
              </Button>
            ))}
        </>
      )}
    </div>
  );
}
