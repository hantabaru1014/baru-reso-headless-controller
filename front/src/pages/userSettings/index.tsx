import { useMemo, useState } from "react";
import { useForm } from "react-hook-form";
import { useTranslation } from "react-i18next";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import type { TFunction } from "i18next";
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

const makePasswordSchema = (t: TFunction) =>
  z
    .object({
      currentPassword: z
        .string()
        .min(1, t("userSettingsPage.currentPasswordRequired")),
      newPassword: z
        .string()
        .min(8, t("userSettingsPage.newPasswordMinLength")),
      confirmPassword: z.string(),
    })
    .refine((data) => data.newPassword === data.confirmPassword, {
      message: t("userSettingsPage.passwordMismatch"),
      path: ["confirmPassword"],
    });

type PasswordFormData = z.infer<ReturnType<typeof makePasswordSchema>>;

export default function UserSettings() {
  const { t } = useTranslation();
  const [success, setSuccess] = useState(false);

  const passwordSchema = useMemo(() => makePasswordSchema(t), [t]);

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
      <Card>
        <CardHeader>
          <CardTitle>{t("userSettingsPage.changePassword")}</CardTitle>
          <CardDescription>
            {t("userSettingsPage.changePasswordDescription")}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
            <TextField
              label={t("userSettingsPage.currentPasswordLabel")}
              type="password"
              {...register("currentPassword")}
              disabled={isPending}
              error={errors.currentPassword?.message}
            />
            <TextField
              label={t("userSettingsPage.newPasswordLabel")}
              type="password"
              {...register("newPassword")}
              disabled={isPending}
              error={errors.newPassword?.message}
            />
            <TextField
              label={t("userSettingsPage.confirmPasswordLabel")}
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
                <AlertDescription>
                  {t("userSettingsPage.passwordChanged")}
                </AlertDescription>
              </Alert>
            )}

            <Button type="submit" disabled={isPending}>
              {isPending ? (
                <>
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  {t("userSettingsPage.changing")}
                </>
              ) : (
                t("userSettingsPage.changePasswordButton")
              )}
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
