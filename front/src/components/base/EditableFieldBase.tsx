import { ButtonGroup, IconButton, Stack } from "@mui/material";
import {
  EditOutlined,
  CheckOutlined,
  CloseOutlined,
} from "@mui/icons-material";

export default function EditableFieldBase({
  editing,
  onEditStart,
  onSave,
  onCancel,
  readonly,
  children,
}: {
  editing: boolean;
  onEditStart?: () => void;
  onSave?: () => void;
  onCancel?: () => void;
  readonly?: boolean;
  children: React.ReactNode;
}) {
  return (
    <Stack direction="row">
      <div style={{ flexGrow: 1 }}>{children}</div>
      {!readonly &&
        (editing ? (
          <ButtonGroup>
            <IconButton aria-label="保存" onClick={onSave}>
              <CheckOutlined />
            </IconButton>
            <IconButton aria-label="キャンセル" onClick={onCancel}>
              <CloseOutlined />
            </IconButton>
          </ButtonGroup>
        ) : (
          <IconButton aria-label="編集" onClick={onEditStart}>
            <EditOutlined />
          </IconButton>
        ))}
    </Stack>
  );
}
