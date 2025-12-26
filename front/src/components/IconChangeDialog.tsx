import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
  Button,
  DialogClose,
} from "./ui";
import { useCallback, useRef, useState, useEffect } from "react";
import { ImageIcon, Upload } from "lucide-react";
import { cn } from "@/libs/cssUtils";
import { processIconImage, createImageUrl } from "@/libs/imageUtils";
import { ResoniteUserIcon } from "./ResoniteUserIcon";
import Cropper, { type Area } from "react-easy-crop";

interface IconChangeDialogProps {
  open: boolean;
  onClose?: () => void;
  currentIconUrl?: string;
  onUpload: (iconData: Uint8Array) => Promise<void>;
  isUploading?: boolean;
}

export function IconChangeDialog({
  open,
  onClose,
  currentIconUrl,
  onUpload,
  isUploading = false,
}: IconChangeDialogProps) {
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [imageUrl, setImageUrl] = useState<string | null>(null);
  const [crop, setCrop] = useState({ x: 0, y: 0 });
  const [zoom, setZoom] = useState(1);
  const [croppedAreaPixels, setCroppedAreaPixels] = useState<Area | null>(null);
  const [isDragOver, setIsDragOver] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Clean up image URL when dialog closes or file changes
  useEffect(() => {
    return () => {
      if (imageUrl) {
        URL.revokeObjectURL(imageUrl);
      }
    };
  }, [imageUrl]);

  // Reset state when dialog opens/closes
  useEffect(() => {
    if (!open) {
      if (imageUrl) {
        URL.revokeObjectURL(imageUrl);
      }
      setImageUrl(null);
      setCrop({ x: 0, y: 0 });
      setZoom(1);
      setCroppedAreaPixels(null);
      setError(null);
    }
  }, [open]);

  const onCropComplete = useCallback((_: Area, croppedAreaPixels: Area) => {
    setCroppedAreaPixels(croppedAreaPixels);
  }, []);

  const loadFile = useCallback((file: File) => {
    // Validate file size (10MB max)
    if (file.size > 10 * 1024 * 1024) {
      setError("ファイルサイズは10MB以下にしてください");
      return;
    }

    // Validate file type
    if (!file.type.startsWith("image/")) {
      setError("画像ファイルを選択してください");
      return;
    }

    setError(null);

    // Revoke old image URL
    setImageUrl((oldUrl) => {
      if (oldUrl) URL.revokeObjectURL(oldUrl);
      return createImageUrl(file);
    });
    setCrop({ x: 0, y: 0 });
    setZoom(1);
  }, []);

  const handleFileSelect = useCallback(
    (event: React.ChangeEvent<HTMLInputElement>) => {
      const file = event.target.files?.[0];
      if (file) {
        loadFile(file);
      }
      // Reset input so same file can be selected again
      event.target.value = "";
    },
    [loadFile],
  );

  const handleDragOver = useCallback((event: React.DragEvent) => {
    event.preventDefault();
    event.stopPropagation();
    setIsDragOver(true);
  }, []);

  const handleDragLeave = useCallback((event: React.DragEvent) => {
    event.preventDefault();
    event.stopPropagation();
    setIsDragOver(false);
  }, []);

  const handleDrop = useCallback(
    (event: React.DragEvent) => {
      event.preventDefault();
      event.stopPropagation();
      setIsDragOver(false);

      const file = event.dataTransfer.files?.[0];
      if (file) {
        loadFile(file);
      }
    },
    [loadFile],
  );

  const handleUpload = useCallback(async () => {
    if (!imageUrl || !croppedAreaPixels) return;

    try {
      const blob = await processIconImage(imageUrl, croppedAreaPixels);
      const arrayBuffer = await blob.arrayBuffer();
      await onUpload(new Uint8Array(arrayBuffer));
      onClose?.();
    } catch (e) {
      setError(e instanceof Error ? e.message : "アップロードに失敗しました");
    }
  }, [imageUrl, croppedAreaPixels, onUpload, onClose]);

  return (
    <Dialog open={open} onOpenChange={(isOpen) => !isOpen && onClose?.()}>
      <DialogContent className="sm:max-w-[480px]">
        <DialogHeader>
          <DialogTitle>アイコンを変更</DialogTitle>
        </DialogHeader>

        <div className="space-y-4">
          {/* Current icon display */}
          {!imageUrl && (
            <div className="flex justify-center">
              <div className="text-center space-y-2">
                <p className="text-sm text-muted-foreground">現在のアイコン</p>
                <ResoniteUserIcon
                  iconUrl={currentIconUrl}
                  alt="現在のアイコン"
                  className="size-20 mx-auto"
                />
              </div>
            </div>
          )}

          {/* Cropper area */}
          {imageUrl && (
            <div className="relative w-full h-64 bg-muted rounded-lg overflow-hidden">
              <Cropper
                image={imageUrl}
                crop={crop}
                zoom={zoom}
                aspect={1}
                onCropChange={setCrop}
                onCropComplete={onCropComplete}
                onZoomChange={setZoom}
              />
            </div>
          )}

          {/* Zoom slider */}
          {imageUrl && (
            <div className="flex items-center gap-3 px-2">
              <span className="text-sm text-muted-foreground">ズーム</span>
              <input
                type="range"
                min={1}
                max={3}
                step={0.1}
                value={zoom}
                onChange={(e) => setZoom(Number(e.target.value))}
                className="flex-1 accent-primary"
              />
            </div>
          )}

          {/* Drop zone / Change image button */}
          {!imageUrl ? (
            <div
              className={cn(
                "border-2 border-dashed rounded-lg p-6 text-center cursor-pointer transition-colors",
                isDragOver
                  ? "border-primary bg-primary/10"
                  : "border-muted-foreground/25 hover:border-muted-foreground/50",
              )}
              onClick={() => fileInputRef.current?.click()}
              onDragOver={handleDragOver}
              onDragLeave={handleDragLeave}
              onDrop={handleDrop}
            >
              <input
                type="file"
                ref={fileInputRef}
                onChange={handleFileSelect}
                accept="image/png,image/jpeg,image/gif,image/webp"
                className="hidden"
              />
              <div className="flex flex-col items-center gap-2">
                <ImageIcon className="size-8 text-muted-foreground" />
                <div className="text-sm text-muted-foreground">
                  <span className="text-primary font-medium">
                    ファイルを選択
                  </span>
                  するか、ここにドラッグ&ドロップ
                </div>
                <div className="text-xs text-muted-foreground">
                  PNG, JPG, GIF, WebP (最大10MB)
                </div>
              </div>
            </div>
          ) : (
            <div className="flex justify-center">
              <Button
                variant="outline"
                size="sm"
                onClick={() => fileInputRef.current?.click()}
              >
                別の画像を選択
              </Button>
              <input
                type="file"
                ref={fileInputRef}
                onChange={handleFileSelect}
                accept="image/png,image/jpeg,image/gif,image/webp"
                className="hidden"
              />
            </div>
          )}

          {/* Error message */}
          {error && (
            <div className="text-sm text-destructive text-center">{error}</div>
          )}

          {/* Info */}
          <div className="text-xs text-muted-foreground text-center">
            ドラッグして切り抜き範囲を調整できます（256x256にリサイズされます）
          </div>
        </div>

        <DialogFooter>
          <DialogClose asChild>
            <Button variant="outline" disabled={isUploading}>
              キャンセル
            </Button>
          </DialogClose>
          <Button
            onClick={handleUpload}
            disabled={!imageUrl || !croppedAreaPixels || isUploading}
          >
            {isUploading ? (
              <>
                <Upload className="size-4 mr-2 animate-bounce" />
                アップロード中...
              </>
            ) : (
              "変更"
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
