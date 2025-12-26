import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useMutation } from "@connectrpc/connect-query";
import { changePassword } from "../../../pbgen/hdlctrl/v1/user-UserService_connectquery";
import {
  Button,
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  CardDescription,
  Alert,
  AlertDescription,
} from "@/components/ui";
import { TextField } from "@/components/base";
import { Loader2, CheckCircle } from "lucide-react";

const passwordSchema = z
  .object({
    currentPassword: z.string().min(1, "現在のパスワードを入力してください"),
    newPassword: z.string().min(8, "パスワードは8文字以上である必要があります"),
    confirmPassword: z.string(),
  })
  .refine((data) => data.newPassword === data.confirmPassword, {
    message: "パスワードが一致しません",
    path: ["confirmPassword"],
  });

type PasswordFormData = z.infer<typeof passwordSchema>;

export default function UserSettings() {
  const [success, setSuccess] = useState(false);

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors },
  } = useForm<PasswordFormData>({
    resolver: zodResolver(passwordSchema),
  });

  const { mutateAsync, isPending, error } = useMutation(changePassword);

  const onSubmit = async (data: PasswordFormData) => {
    setSuccess(false);
    try {
      await mutateAsync({
        currentPassword: data.currentPassword,
        newPassword: data.newPassword,
      });
      setSuccess(true);
      reset();
      setTimeout(() => setSuccess(false), 3000);
    } catch {
      // エラーはuseMutationが管理
    }
  };

  return (
    <div className="container max-w-2xl mx-auto py-6">
      <h1 className="text-2xl font-bold mb-6">ユーザー設定</h1>

      <Card>
        <CardHeader>
          <CardTitle>パスワード変更</CardTitle>
          <CardDescription>
            新しいパスワードを入力してください。
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
            <TextField
              label="現在のパスワード"
              type="password"
              {...register("currentPassword")}
              disabled={isPending}
              error={errors.currentPassword?.message}
            />
            <TextField
              label="新しいパスワード"
              type="password"
              {...register("newPassword")}
              disabled={isPending}
              error={errors.newPassword?.message}
            />
            <TextField
              label="新しいパスワード（確認）"
              type="password"
              {...register("confirmPassword")}
              disabled={isPending}
              error={errors.confirmPassword?.message}
            />

            {error && (
              <Alert variant="destructive">
                <AlertDescription>{error.message}</AlertDescription>
              </Alert>
            )}

            {success && (
              <Alert>
                <CheckCircle className="h-4 w-4" />
                <AlertDescription>パスワードを変更しました。</AlertDescription>
              </Alert>
            )}

            <Button type="submit" disabled={isPending}>
              {isPending ? (
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
