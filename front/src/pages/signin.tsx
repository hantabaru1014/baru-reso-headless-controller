"use client";
import { SignInPage } from "@toolpad/core/SignInPage";
import { Navigate, useNavigate } from "react-router";
import { useMutation } from "@connectrpc/connect-query";
import { getTokenByPassword } from "../../pbgen/hdlctrl/v1/user-UserService_connectquery";
import { useNotifications } from "@toolpad/core/useNotifications";
import { useAtom } from "jotai";
import { Session, sessionAtom } from "../atoms/sessionAtom";

export default function SignIn() {
  const [session, setSession] = useAtom(sessionAtom);
  const navigate = useNavigate();
  const notifications = useNotifications();
  const { mutate } = useMutation(getTokenByPassword);

  if (session) {
    return <Navigate to="/" />;
  }

  return (
    <SignInPage
      providers={[{ id: "credentials", name: "Credentials" }]}
      signIn={(provider, formData, callbackUrl) => {
        if (provider.id === "credentials") {
          const email = formData?.get("email") as string;
          const password = formData?.get("password") as string;

          mutate(
            {
              id: email,
              password,
            },
            {
              onSuccess(data, variables) {
                const userSession: Session = {
                  token: data.token,
                  user: {
                    name: variables.id,
                    email: variables.id,
                  },
                };
                setSession(userSession);
                navigate(callbackUrl || "/", { replace: true });
              },
              onError(error) {
                const message =
                  error instanceof Error ? error.message : "An error occurred";
                notifications.show(message, { severity: "error" });
              },
            },
          );
        }
      }}
    />
  );
}
