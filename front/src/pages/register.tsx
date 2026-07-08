import { useState, useEffect, useMemo } from "react";
import { Navigate, useNavigate, useParams } from "react-router";
import { useForm } from "react-hook-form";
import { useTranslation } from "react-i18next";
import { useAtom } from "jotai";
import { sessionAtom, sessionRefreshTokenAtom } from "../atoms/sessionAtom";
import { createConnectTransport } from "@connectrpc/connect-web";
import { callUnaryMethod } from "@connectrpc/connect-query";
import { jwtDecode } from "jwt-decode";
import { UserService } from "../../pbgen/hdlctrl/v1/user_pb";
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
import { Loader2 } from "lucide-react";
import { TextField } from "@/components/base";
import { ResoniteUserIcon } from "@/components/ResoniteUserIcon";

interface RegisterForm {
  userId: string;
  password: string;
  confirmPassword: string;
}

interface JwtPayload {
  user_id: string;
  resonite_id: string;
  icon_url: string;
}

export default function Register() {
  const { t } = useTranslation();
  const { token } = useParams<{ token: string }>();
  // personal_role_id は token と紐付けて DB に永続化されているので URL 経由では受け取らない.
  const [session, setSession] = useAtom(sessionAtom);
  const [, setRefreshToken] = useAtom(sessionRefreshTokenAtom);
  const navigate = useNavigate();

  const [error, setError] = useState<string | undefined>();
  const [isLoading, setIsLoading] = useState(false);
  const [isValidating, setIsValidating] = useState(true);
  const [isValidToken, setIsValidToken] = useState(false);
  const [resoniteId, setResoniteId] = useState<string>("");
  const [resoniteUserName, setResoniteUserName] = useState<string>("");
  const [iconUrl, setIconUrl] = useState<string>("");

  const transport = useMemo(() => createConnectTransport({ baseUrl: "/" }), []);

  const {
    register,
    handleSubmit,
    watch,
    formState: { errors },
  } = useForm<RegisterForm>();

  const password = watch("password");

  useEffect(() => {
    const validateToken = async () => {
      if (!token) {
        setIsValidating(false);
        return;
      }

      try {
        const response = await callUnaryMethod(
          transport,
          UserService.method.validateRegistrationToken,
          { token },
        );

        if (response.valid) {
          setIsValidToken(true);
          setResoniteId(response.resoniteId);
          setResoniteUserName(response.resoniteUserName);
          setIconUrl(response.iconUrl);
        }
      } catch (err) {
        console.error("Token validation failed:", err);
      } finally {
        setIsValidating(false);
      }
    };

    validateToken();
  }, [token, transport]);

  if (session) {
    return <Navigate to="/" />;
  }

  if (isValidating) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-background p-4">
        <Card className="w-full max-w-md">
          <CardContent className="pt-6">
            <div className="flex items-center justify-center">
              <Loader2 className="h-8 w-8 animate-spin" />
              <span className="ml-2">{t("registerPage.validatingToken")}</span>
            </div>
          </CardContent>
        </Card>
      </div>
    );
  }

  if (!isValidToken) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-background p-4">
        <Card className="w-full max-w-md">
          <CardHeader className="space-y-1">
            <CardTitle className="text-2xl font-bold text-center text-destructive">
              {t("registerPage.invalidLink")}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <Alert variant="destructive">
              <AlertDescription>
                {t("registerPage.invalidLinkDescription")}
              </AlertDescription>
            </Alert>
            <Button
              className="w-full mt-4"
              variant="outline"
              onClick={() => navigate("/sign-in")}
            >
              {t("registerPage.toSignInPage")}
            </Button>
          </CardContent>
        </Card>
      </div>
    );
  }

  const onSubmit = async (data: RegisterForm) => {
    if (!token) return;

    setIsLoading(true);
    setError(undefined);

    try {
      const response = await callUnaryMethod(
        transport,
        UserService.method.registerWithToken,
        {
          token,
          userId: data.userId,
          password: data.password,
        },
      );

      const decoded = jwtDecode<JwtPayload>(response.token);
      setSession({
        token: response.token,
        user: {
          id: decoded.user_id,
          iconUrl: decoded.icon_url,
          resoniteId: decoded.resonite_id,
        },
      });
      setRefreshToken(response.refreshToken);
      navigate("/", { replace: true });
    } catch (err) {
      console.error("Registration failed:", err);
      setError(t("registerPage.registrationFailed"));
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div className="min-h-screen flex items-center justify-center bg-background p-4">
      <Card className="w-full max-w-md">
        <CardHeader className="space-y-1">
          <div className="flex flex-col items-center gap-3">
            <ResoniteUserIcon
              iconUrl={iconUrl}
              alt={resoniteUserName}
              className="size-16"
            />
            <CardTitle className="text-2xl font-bold text-center">
              {t("registerPage.welcome", {
                name: resoniteUserName || resoniteId,
              })}
            </CardTitle>
            <CardDescription className="text-center text-muted-foreground text-xs">
              {resoniteId}
            </CardDescription>
          </div>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
            <TextField
              label={t("registerPage.userIdLabel")}
              type="text"
              {...register("userId", {
                required: t("registerPage.userIdRequired"),
                minLength: {
                  value: 3,
                  message: t("registerPage.userIdMinLength"),
                },
              })}
              disabled={isLoading}
              error={errors.userId?.message}
            />
            <TextField
              label={t("registerPage.passwordLabel")}
              type="password"
              {...register("password", {
                required: t("registerPage.passwordRequired"),
                minLength: {
                  value: 8,
                  message: t("registerPage.passwordMinLength"),
                },
              })}
              disabled={isLoading}
              error={errors.password?.message}
            />
            <TextField
              label={t("registerPage.confirmPasswordLabel")}
              type="password"
              {...register("confirmPassword", {
                required: t("registerPage.confirmPasswordRequired"),
                validate: (value) =>
                  value === password || t("registerPage.passwordMismatch"),
              })}
              disabled={isLoading}
              error={errors.confirmPassword?.message}
            />
            {error && (
              <Alert variant="destructive">
                <AlertDescription>{error}</AlertDescription>
              </Alert>
            )}
            <Button type="submit" className="w-full" disabled={isLoading}>
              {isLoading ? (
                <>
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  {t("registerPage.registering")}
                </>
              ) : (
                t("registerPage.register")
              )}
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
