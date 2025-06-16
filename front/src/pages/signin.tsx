"use client";
import { useState } from "react";
import { Navigate, useNavigate } from "react-router";
import { useForm } from "react-hook-form";
import { useAtom } from "jotai";
import { sessionAtom } from "../atoms/sessionAtom";
import { useAuth } from "../hooks/useAuth";
import {
  Button,
  Input,
  Label,
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  Alert,
  AlertDescription,
} from "@/components/ui";
import { Loader2 } from "lucide-react";

interface SignInForm {
  email: string;
  password: string;
}

export default function SignIn() {
  const [session] = useAtom(sessionAtom);
  const navigate = useNavigate();
  const { signIn } = useAuth("/");
  const query = new URLSearchParams(location.search);
  const queryCallbackUrl = query.get("callbackUrl");

  const [error, setError] = useState<string | undefined>();
  const [isLoading, setIsLoading] = useState(false);

  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<SignInForm>();

  if (session) {
    return <Navigate to={queryCallbackUrl || "/"} />;
  }

  const onSubmit = async (data: SignInForm) => {
    setIsLoading(true);
    setError(undefined);

    const response = await signIn(data.email, data.password);
    if (response.ok) {
      navigate(queryCallbackUrl || "/", { replace: true });
    } else {
      setError(response.error);
    }
    setIsLoading(false);
  };

  return (
    <div className="min-h-screen flex items-center justify-center bg-background p-4">
      <Card className="w-full max-w-md">
        <CardHeader className="space-y-1">
          <CardTitle className="text-2xl font-bold text-center">
            サインイン
          </CardTitle>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="email">ID</Label>
              <Input
                id="email"
                type="email"
                {...register("email", {
                  required: "IDは必須です",
                })}
                disabled={isLoading}
                className={errors.email ? "border-destructive" : ""}
              />
              {errors.email && (
                <p className="text-sm text-destructive">
                  {errors.email.message}
                </p>
              )}
            </div>
            <div className="space-y-2">
              <Label htmlFor="password">パスワード</Label>
              <Input
                id="password"
                type="password"
                {...register("password", {
                  required: "パスワードは必須です",
                })}
                disabled={isLoading}
                className={errors.password ? "border-destructive" : ""}
              />
              {errors.password && (
                <p className="text-sm text-destructive">
                  {errors.password.message}
                </p>
              )}
            </div>
            {error && (
              <Alert variant="destructive">
                <AlertDescription>{error}</AlertDescription>
              </Alert>
            )}
            <Button type="submit" className="w-full" disabled={isLoading}>
              {isLoading ? (
                <>
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  サインイン中...
                </>
              ) : (
                "サインイン"
              )}
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
