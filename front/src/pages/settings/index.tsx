import { useState } from "react";
import { useForm } from "react-hook-form";
import { useAuth } from "@/hooks/useAuth";
import {
  Button,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
  Alert,
  AlertDescription,
} from "@/components/ui";
import { TextField } from "@/components/base";
import { Loader2 } from "lucide-react";

interface ChangePasswordForm {
  currentPassword: string;
  newPassword: string;
  confirmPassword: string;
}

export default function Settings() {
  const { configuredFetch } = useAuth("/");
  const [error, setError] = useState<string | undefined>();
  const [success, setSuccess] = useState<string | undefined>();
  const [isLoading, setIsLoading] = useState(false);

  const {
    register,
    handleSubmit,
    reset,
    watch,
    formState: { errors },
  } = useForm<ChangePasswordForm>();

  const newPassword = watch("newPassword");

  const onSubmit = async (data: ChangePasswordForm) => {
    setIsLoading(true);
    setError(undefined);
    setSuccess(undefined);

    try {
      const response = await configuredFetch("/api/change-password", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          current_password: data.currentPassword,
          new_password: data.newPassword,
        }),
      });

      if (response.ok) {
        setSuccess("パスワードを変更しました");
        reset();
      } else {
        const errorData = await response.json().catch(() => ({}));
        setError(errorData.error || "パスワードの変更に失敗しました");
      }
    } catch {
      setError("パスワードの変更に失敗しました");
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div className="max-w-2xl mx-auto space-y-6">
      <div>
        <h2 className="text-2xl font-bold tracking-tight">設定</h2>
        <p className="text-muted-foreground">アカウント設定を管理します</p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>パスワード変更</CardTitle>
          <CardDescription>
            ログインパスワードを変更します
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
            <TextField
              label="現在のパスワード"
              type="password"
              {...register("currentPassword", {
                required: "現在のパスワードは必須です",
              })}
              disabled={isLoading}
              error={errors.currentPassword?.message}
            />
            <TextField
              label="新しいパスワード"
              type="password"
              {...register("newPassword", {
                required: "新しいパスワードは必須です",
                minLength: {
                  value: 8,
                  message: "パスワードは8文字以上で入力してください",
                },
              })}
              disabled={isLoading}
              error={errors.newPassword?.message}
            />
            <TextField
              label="新しいパスワード（確認）"
              type="password"
              {...register("confirmPassword", {
                required: "確認用パスワードは必須です",
                validate: (value) =>
                  value === newPassword || "パスワードが一致しません",
              })}
              disabled={isLoading}
              error={errors.confirmPassword?.message}
            />
            {error && (
              <Alert variant="destructive">
                <AlertDescription>{error}</AlertDescription>
              </Alert>
            )}
            {success && (
              <Alert>
                <AlertDescription>{success}</AlertDescription>
              </Alert>
            )}
            <Button type="submit" disabled={isLoading}>
              {isLoading ? (
                <>
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  変更中...
                </>
              ) : (
                "パスワードを変更"
              )}
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
